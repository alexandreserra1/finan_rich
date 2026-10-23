package advisor

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// Anonimização para IA na nuvem: antes de sair do aparelho, lojas,
// estabelecimentos, dívidas, credores e metas viram apelidos ("Loja 3",
// "Dívida 1") e as notas ficam de fora. A IA vê valores, datas e categorias,
// que é o que ela precisa para aconselhar, mas não sabe onde a pessoa compra
// nem com quem tem dívida. Na volta, o app troca os apelidos pelos nomes.

// Anonymizer guarda os apelidos de uma conversa (só na memória do aparelho).
type Anonymizer struct {
	phrases []phrase                     // nomes como a pessoa escreve → apelido (para a pergunta)
	display map[string]string            // apelido → nome para mostrar
	records map[string]map[string]string // ID do registro → campo → texto original
	alias   map[string]map[string]string // ID do registro → campo → apelido usado
}

type phrase struct {
	text  []rune // já sem acento e minúsculo
	alias string
}

// Anonymize devolve uma cópia dos dados com apelidos no lugar dos nomes.
func Anonymize(l domain.Ledger) (domain.Ledger, *Anonymizer) {
	a := &Anonymizer{display: map[string]string{}, records: map[string]map[string]string{}, alias: map[string]map[string]string{}}
	out := domain.Ledger{Prefs: l.Prefs} // notas ficam de fora

	// Categorias padrão são genéricas e ficam; as criadas pela pessoa ("Pensão
	// Maria", "Psicóloga Dra. Ana") viram "Categoria N".
	generic := map[string]bool{}
	for _, c := range domain.DefaultCategories(0) {
		generic[fold(c.Name)] = true
	}
	catNames := map[string]bool{}
	custom := 0
	for _, c := range l.Categories {
		catNames[fold(c.Name)] = true
		if !generic[fold(c.Name)] {
			custom++
			al := fmt.Sprintf("Categoria %d", custom)
			a.named(c.ID, "name", c.Name, al)
			// "quanto gastei com a psicóloga?" também acha "Psicóloga Dra. Ana".
			if w := strings.Fields(c.Name); len(w) > 1 && len([]rune(w[0])) >= 5 && !generic[fold(w[0])] {
				a.addPhrase(w[0], al)
			}
			c.Name = al
		}
		out.Categories = append(out.Categories, c)
	}
	// Uma palavra que é nome de categoria ("mercado", "aluguel") não vira
	// apelido na pergunta: senão "categoria Mercado" viraria "categoria Loja AB".
	known := func(text, alias string) {
		if !catNames[fold(text)] {
			a.addPhrase(text, alias)
		}
	}

	// Estabelecimentos: primeiro descobre os grupos, para os apelidos terem
	// todos o mesmo tamanho ("Loja AB" nunca é pedaço de "Loja ABC").
	type group struct{ key, show, kind string }
	groupOf := func(desc, kind string) group {
		key, name := importer.PlaceKey(desc)
		if key == "" { // "C&A", "Transferência PIX": vale a descrição inteira
			return group{"#" + fold(desc), desc, kind}
		}
		return group{key, name, kind}
	}
	var order []group
	seen := map[string]bool{}
	visit := func(desc, kind string) {
		if g := groupOf(desc, kind); !seen[g.key] {
			seen[g.key] = true
			order = append(order, g)
		}
	}
	for _, t := range l.Transactions {
		visit(t.Description, t.Kind)
	}
	for _, r := range l.Recurring {
		visit(r.Description, r.Kind)
	}
	codes := aliasCodes(len(order))
	byKey := map[string]string{} // grupo → apelido
	for i, g := range order {
		prefix := "Loja"
		if g.kind == domain.KindEntrada {
			prefix = "Fonte"
		}
		al := prefix + " " + codes[i]
		byKey[g.key] = al
		a.display[al] = g.show
		if !strings.HasPrefix(g.key, "#") {
			known(g.key, al)
		}
		known(g.show, al)
	}
	shop := func(id, desc, kind string) string {
		al := byKey[groupOf(desc, kind).key]
		// A descrição inteira também é trocada: se ela voltar numa resposta
		// ("PIX ENVIADO 123.456.789-00 JOAO SILVA"), não vaza na próxima pergunta.
		known(desc, al)
		if key := importer.DescKey(desc); key != "" {
			known(key, al)
		}
		a.keep(id, "description", desc, al)
		return al
	}
	for _, t := range l.Transactions {
		t.Description = shop(t.ID, t.Description, t.Kind)
		out.Transactions = append(out.Transactions, t)
	}
	for _, r := range l.Recurring {
		r.Description = shop(r.ID, r.Description, r.Kind)
		out.Recurring = append(out.Recurring, r)
	}
	creditors := map[string]string{}
	for i, d := range l.Debts {
		al := fmt.Sprintf("Dívida %d", i+1)
		a.named(d.ID, "name", d.Name, al)
		d.Name = al
		if d.Creditor != "" {
			c, ok := creditors[fold(d.Creditor)]
			if !ok {
				c = fmt.Sprintf("Credor %d", len(creditors)+1)
				creditors[fold(d.Creditor)] = c
				a.named("", "", d.Creditor, c)
			}
			a.keep(d.ID, "creditor", d.Creditor, c)
			d.Creditor = c
		}
		out.Debts = append(out.Debts, d)
	}
	for i, g := range l.Goals {
		al := fmt.Sprintf("Meta %d", i+1)
		a.named(g.ID, "name", g.Name, al)
		g.Name = al
		out.Goals = append(out.Goals, g)
	}
	// Nomes mais longos primeiro: "cartão nubank" antes de "cartão".
	slices.SortStableFunc(a.phrases, func(x, y phrase) int { return len(y.text) - len(x.text) })
	return out, a
}

// aliasCodes devolve n códigos de letras, todos do mesmo tamanho ("AB", "AC"...),
// pulando os que a comparação de descrições ignora ("SA", "DE"...).
func aliasCodes(n int) []string {
	size := 2
	for float64(n) > 0.9*math.Pow(26, float64(size)) {
		size++
	}
	out := make([]string, 0, n)
	for i := 0; len(out) < n; i++ {
		code := make([]byte, size)
		for j, v := size-1, i; j >= 0; j, v = j-1, v/26 {
			code[j] = byte('A' + v%26)
		}
		if importer.DescKey("loja "+string(code)) == "loja "+strings.ToLower(string(code)) {
			out = append(out, string(code))
		}
	}
	return out
}

func (a *Anonymizer) named(id, field, name, alias string) {
	a.display[alias] = name
	a.addPhrase(name, alias)
	if id != "" {
		a.keep(id, field, name, alias)
	}
}

func (a *Anonymizer) keep(id, field, original, alias string) {
	if a.records[id] == nil {
		a.records[id], a.alias[id] = map[string]string{}, map[string]string{}
	}
	a.records[id][field], a.alias[id][field] = original, alias
}

func (a *Anonymizer) addPhrase(text, alias string) {
	if r := foldRunes(text); len(r) > 0 {
		a.phrases = append(a.phrases, phrase{r, alias})
	}
}

// foldRunes: minúsculas e sem acento, uma runa por runa (mantém as posições).
func foldRunes(s string) []rune {
	out := []rune(s)
	for i, r := range out {
		out[i] = []rune(fold(string(unicode.ToLower(r))) + " ")[0]
	}
	return out
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// Text troca, num texto da pessoa (pergunta, histórico), os nomes conhecidos
// pelos apelidos. Só troca palavras inteiras.
func (a *Anonymizer) Text(s string) string {
	src, low := []rune(s), foldRunes(s)
	var out []rune
	for i := 0; i < len(src); {
		matched := false
		if i == 0 || !isWordRune(low[i-1]) {
			for _, p := range a.phrases {
				end := i + len(p.text)
				if end <= len(low) && slices.Equal(low[i:end], p.text) && (end == len(low) || !isWordRune(low[end])) {
					out = append(out, []rune(p.alias)...)
					i, matched = end, true
					break
				}
			}
		}
		if !matched {
			out = append(out, src[i])
			i++
		}
	}
	return string(out)
}

// Reveal troca os apelidos de volta pelos nomes (resposta da IA). Os espaços
// especiais (o gpt-oss usa o inseparável estreito, U+202F) viram espaço comum
// antes, senão "Loja AB" não seria reconhecido.
func (a *Anonymizer) Reveal(s string) string {
	s = strings.Map(func(r rune) rune {
		if r != ' ' && unicode.Is(unicode.Zs, r) {
			return ' '
		}
		return r
	}, s)
	if len(a.display) == 0 {
		return s
	}
	aliases := make([]string, 0, len(a.display))
	byLower := make(map[string]string, len(a.display))
	for al, name := range a.display {
		aliases = append(aliases, regexp.QuoteMeta(al))
		byLower[strings.ToLower(al)] = name
	}
	// "Loja 12" antes de "Loja 1"; palavra inteira e sem ligar para maiúsculas
	// (o modelo às vezes escreve "loja AB"), para "Dívida 12" não virar "Dívida 1" + "2".
	slices.SortFunc(aliases, func(x, y string) int { return len(y) - len(x) })
	re := regexp.MustCompile(`(?i)\b(?:` + strings.Join(aliases, "|") + `)\b`)
	return re.ReplaceAllStringFunc(s, func(m string) string { return byLower[strings.ToLower(m)] })
}

// RevealProposal devolve a proposta com os nomes verdadeiros. Campos que a IA
// não mudou voltam exatamente como estavam no registro (pelo ID); o resto
// passa por Reveal. Assim um apelido nunca é gravado nos dados.
func (a *Anonymizer) RevealProposal(p Proposal) Proposal {
	p.Record = a.revealRecord(p.Record)
	if len(p.Before) > 0 {
		p.Before = a.revealRecord(p.Before)
	}
	p.Label = a.Reveal(p.Label)
	return p
}

func (a *Anonymizer) revealRecord(raw json.RawMessage) json.RawMessage {
	var rec map[string]any
	if json.Unmarshal(raw, &rec) != nil {
		return raw
	}
	id, _ := rec["id"].(string)
	for _, field := range []string{"description", "name", "creditor"} {
		v, ok := rec[field].(string)
		if !ok {
			continue
		}
		if a.alias[id][field] == v {
			rec[field] = a.records[id][field]
		} else {
			rec[field] = a.Reveal(v)
		}
	}
	out, _ := json.Marshal(rec)
	return out
}
