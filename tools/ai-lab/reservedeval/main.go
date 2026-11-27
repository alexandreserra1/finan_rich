package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Totals struct {
	Cases       int `json:"cases"`
	FailedCases int `json:"failed_cases"`
	Passed      int `json:"passed"`
	Total       int `json:"total"`
}

type Report struct {
	Synthetic       bool                         `json:"synthetic"`
	Split           string                       `json:"split"`
	Variant         string                       `json:"variant"`
	Mode            string                       `json:"mode"`
	Model           string                       `json:"model"`
	Digest          string                       `json:"digest"`
	GoVersion       string                       `json:"go_version"`
	CodeSHA256      string                       `json:"code_sha256"`
	Inputs          map[string]string            `json:"inputs"`
	ReservedBefore  string                       `json:"reserved_before"`
	ReservedAfter   string                       `json:"reserved_after"`
	PerCaseMS       int64                        `json:"per_case_ms"`
	GlobalMS        int64                        `json:"global_ms"`
	Results         []Result                     `json:"results"`
	Totals          map[string]Totals            `json:"totals"`
	Categories      map[string]map[string]Totals `json:"categories"`
	Step5Complete   bool                         `json:"step5_complete"`
	TrainingAllowed bool                         `json:"training_allowed"`
}

func inside(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func localPath(root, path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("caminho local inválido")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", errors.New("arquivo local ausente")
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil || !inside(root, resolved) {
		return "", errors.New("entrada deve ficar dentro de .artifacts")
	}
	return resolved, nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("não foi possível abrir entrada")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFile {
		return nil, errors.New("entrada não regular ou maior que 8 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFile+1))
	if err != nil || len(data) > maxFile {
		return nil, errors.New("falha de leitura ou limite excedido")
	}
	return data, nil
}

func treeHash(ctx context.Context, root string) (string, error) {
	files := map[string]string{}
	total := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("não foi possível conferir diretório reservado")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || len(files) >= 200 {
			return errors.New("reservado exige até 200 arquivos regulares, sem links")
		}
		data, err := readBounded(path)
		if err != nil {
			return err
		}
		total += len(data)
		if total > 32<<20 {
			return errors.New("reservado excedeu 32 MiB")
		}
		relative, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(relative)] = digest(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("diretório reservado vazio")
	}
	data, _ := json.Marshal(files)
	return digest(data), nil
}

func validateCollection(collection Collection, fixtures Fixtures, hashes map[string]string, variant string) error {
	if !collection.Synthetic || collection.Model != baseline || collection.Digest != baselineDigest ||
		collection.Endpoint != "http://127.0.0.1:11434" || collection.FixturesSHA256 != hashes["fixtures"] ||
		collection.RulesSHA256 != hashes["rules"] || collection.Variant != variant {
		return errors.New("coleta não corresponde ao baseline local, casos, critérios ou wrapper")
	}
	ids := map[string]bool{}
	for _, item := range fixtures.Cases {
		ids[item.ID] = true
	}
	seen := map[string]bool{}
	for _, item := range collection.Cases {
		if !ids[item.ID] || seen[item.ID] || len(item.Rounds) > 8 {
			return errors.New("coleta com casos extras, repetidos ou rodadas demais")
		}
		seen[item.ID] = true
	}
	return nil
}

func aggregate(report *Report) {
	report.Totals = map[string]Totals{}
	report.Categories = map[string]map[string]Totals{}
	for _, result := range report.Results {
		total := report.Totals[result.Source]
		total.Cases++
		total.Passed += result.Passed
		total.Total += result.Total
		if result.Passed != result.Total || result.Status != "ok" {
			total.FailedCases++
		}
		report.Totals[result.Source] = total
		if report.Categories[result.Source] == nil {
			report.Categories[result.Source] = map[string]Totals{}
		}
		for _, criterion := range result.Criteria {
			category := report.Categories[result.Source][criterion.Category]
			category.Total++
			if criterion.Pass {
				category.Passed++
			}
			report.Categories[result.Source][criterion.Category] = category
		}
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("reservedeval", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fixturesPath := flags.String("fixtures", "", "casos sintéticos em .artifacts")
	rulesPath := flags.String("rules", "", "critérios externos em .artifacts")
	reservedPath := flags.String("reserved-dir", "", "dataset reservado, somente leitura")
	split := flags.String("split", "valid", "valid ou test")
	variant := flags.String("variant", "producao", "producao ou cauteloso; nunca escolhido automaticamente")
	mode := flags.String("mode", "app", "app ou replay; não há inferência nesta versão")
	collectedPath := flags.String("collected", "", "respostas coletadas antes, em .artifacts")
	allowCollected := flags.Bool("allow-collected", false, "autoriza somente replay sem rede")
	selectionPath := flags.String("selection", "", "relatório de valid que fixa a variante antes do test")
	perCase := flags.Duration("case-timeout", 5*time.Second, "prazo por resposta")
	global := flags.Duration("timeout", time.Minute, "prazo global")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errors.New("argumentos inválidos; consulte tools/ai-lab/AVALIACAO.md")
	}
	if (*split != "valid" && *split != "test") || (*variant != "producao" && *variant != "cauteloso") ||
		(*mode != "app" && *mode != "replay") || *perCase <= 0 || *perCase > 30*time.Second || *global <= 0 || *global > 5*time.Minute || *perCase > *global {
		return errors.New("modo, split, wrapper ou prazos inválidos")
	}
	if *mode == "replay" && (!*allowCollected || *collectedPath == "") || *mode == "app" && (*allowCollected || *collectedPath != "" || *variant != "producao") {
		return errors.New("replay exige --allow-collected e --collected; app não aceita coleta ou wrapper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *global)
	defer cancel()
	absoluteRoot, err := filepath.Abs(".artifacts")
	if err != nil {
		return errors.New("raiz local inválida")
	}
	root, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return errors.New("execute na raiz do checkout com entradas em .artifacts")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return errors.New("raiz local inválida")
	}
	reserved, err := localPath(root, *reservedPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(reserved)
	if err != nil || !info.IsDir() {
		return errors.New("reserved-dir deve ser diretório")
	}
	before, err := treeHash(ctx, reserved)
	if err != nil {
		return err
	}
	inputs := map[string]string{}
	paths := map[string]string{}
	load := func(name, path string, target any, artifact bool) error {
		var resolved string
		var err error
		if artifact {
			resolved, err = localPath(root, path)
		} else {
			resolved, err = filepath.Abs(path)
		}
		if err != nil {
			return err
		}
		data, err := readBounded(resolved)
		if err != nil {
			return err
		}
		if err := strictJSON(data, target); err != nil {
			return err
		}
		inputs[name], paths[name] = digest(data), resolved
		return nil
	}
	var fixtures Fixtures
	var rules Rules
	if err := load("fixtures", *fixturesPath, &fixtures, true); err != nil {
		return err
	}
	if err := load("rules", *rulesPath, &rules, true); err != nil {
		return err
	}
	if err := validate(fixtures, rules, inputs["fixtures"]); err != nil {
		return err
	}
	count := 0
	for _, item := range fixtures.Cases {
		if item.Split == *split {
			count++
		}
	}
	if count == 0 {
		return errors.New("split sem casos")
	}
	models := map[string]string{}
	if err := load("models", "tools/ai-lab/models.json", &models, false); err != nil {
		return err
	}
	if models[baseline] != baselineDigest {
		return errors.New("digest do baseline diverge de models.json")
	}
	var collection *Collection
	if *mode == "replay" {
		collection = &Collection{}
		if err := load("collected", *collectedPath, collection, true); err != nil {
			return err
		}
		if err := validateCollection(*collection, fixtures, inputs, *variant); err != nil {
			return err
		}
	}
	code, err := codeHash(ctx)
	if err != nil {
		return err
	}
	if *split == "test" && *mode == "replay" {
		var selection Report
		if err := load("selection", *selectionPath, &selection, true); err != nil {
			return err
		}
		if !selection.Synthetic || selection.Split != "valid" || selection.Mode != "replay" || selection.Variant != *variant ||
			selection.Digest != baselineDigest || selection.Inputs["fixtures"] != inputs["fixtures"] || selection.Inputs["rules"] != inputs["rules"] ||
			selection.ReservedBefore != before || selection.ReservedAfter != before || selection.CodeSHA256 != code || len(selection.Results) == 0 {
			return errors.New("test exige relatório valid do mesmo contrato, código, reservado e variante")
		}
	}
	report := Report{Synthetic: true, Split: *split, Variant: *variant, Mode: *mode, Model: baseline, Digest: baselineDigest,
		GoVersion: runtime.Version(), CodeSHA256: code, Inputs: inputs, ReservedBefore: before, PerCaseMS: perCase.Milliseconds(), GlobalMS: global.Milliseconds()}
	report.Results = evaluate(ctx, fixtures, rules, *split, *variant, collection, *perCase)
	aggregate(&report)
	for name, path := range paths {
		data, err := readBounded(path)
		if err != nil || digest(data) != inputs[name] {
			return errors.New("entrada mudou durante avaliação; relatório recusado")
		}
	}
	report.ReservedAfter, err = treeHash(ctx, reserved)
	if err != nil || report.ReservedAfter != before {
		return errors.New("dataset reservado mudou ou não pôde ser conferido; relatório recusado")
	}
	if ctx.Err() != nil {
		return errors.New("prazo global excedido; relatório não publicado")
	}
	outputRoot := filepath.Join(root, "ai-lab-reserved-runs")
	if inside(reserved, outputRoot) {
		return errors.New("saída não pode ficar dentro do dataset reservado")
	}
	if err := os.MkdirAll(outputRoot, 0700); err != nil {
		return errors.New("não foi possível criar pasta de resultados")
	}
	resolvedOutput, err := filepath.EvalSymlinks(outputRoot)
	if err != nil || resolvedOutput != outputRoot {
		return errors.New("pasta de saída não aceita links")
	}
	directory, err := os.MkdirTemp(outputRoot, "run-")
	if err != nil {
		return errors.New("não foi possível criar saída nova")
	}
	file, err := os.OpenFile(filepath.Join(directory, "report.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("não foi possível criar relatório exclusivo")
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(report)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		return errors.New("não foi possível gravar relatório completo")
	}
	fmt.Fprintln(out, filepath.Join(directory, "report.json"))
	for _, total := range report.Totals {
		if total.FailedCases > 0 {
			return errors.New("avaliação tem falhas; consulte o relatório sem conteúdo financeiro")
		}
	}
	return nil
}

func codeHash(ctx context.Context) (string, error) {
	paths := []string{"internal/advisor", "internal/domain", "internal/finance", "internal/llm", "internal/money", "internal/rates", "tools/ai-lab/reservedeval"}
	sort.Strings(paths)
	parts := []string{runtime.Version()}
	for _, path := range paths {
		hash, err := treeHash(ctx, path)
		if err != nil {
			return "", err
		}
		parts = append(parts, path+":"+hash)
	}
	return digest([]byte(strings.Join(parts, "\n"))), nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
