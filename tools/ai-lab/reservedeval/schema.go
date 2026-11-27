package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/rates"
)

const maxFile = 8 << 20

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type History struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Case struct {
	ID       string         `json:"id"`
	Family   string         `json:"family"`
	Split    string         `json:"split"`
	Ledger   *domain.Ledger `json:"ledger"`
	Today    string         `json:"today"`
	Agent    string         `json:"agent"`
	Question string         `json:"question"`
	History  []History      `json:"history,omitempty"`
	Rates    *rates.Rates   `json:"rates,omitempty"`
}

type Fixtures struct {
	Synthetic bool   `json:"synthetic"`
	Cases     []Case `json:"cases"`
}

type Criterion struct {
	ID        string              `json:"id"`
	Category  string              `json:"category"`
	Contains  string              `json:"contains,omitempty"`
	Forbids   string              `json:"forbids,omitempty"`
	Calls     *[]ExpectedCall     `json:"calls,omitempty"`
	Proposals *[]ExpectedProposal `json:"proposals,omitempty"`
}

type ExpectedCall struct {
	Name  string          `json:"name"`
	Args  json.RawMessage `json:"args"`
	Error bool            `json:"error,omitempty"`
}

type ExpectedProposal struct {
	Action string          `json:"action"`
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
	Before json.RawMessage `json:"before,omitempty"`
}

type CaseRules struct {
	ID       string      `json:"id"`
	Criteria []Criterion `json:"criteria"`
}

type Rules struct {
	Synthetic      bool        `json:"synthetic"`
	FixturesSHA256 string      `json:"fixtures_sha256"`
	Cases          []CaseRules `json:"cases"`
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func strictJSON(data []byte, target any) error {
	if len(data) > maxFile {
		return errors.New("JSON excedeu o limite")
	}
	tokens := json.NewDecoder(bytes.NewReader(data))
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 32 {
			return errors.New("JSON profundo demais")
		}
		token, err := tokens.Token()
		if err != nil {
			return err
		}
		if opening, ok := token.(json.Delim); ok {
			keys := map[string]bool{}
			for tokens.More() {
				if opening == '{' {
					key, err := tokens.Token()
					if err != nil {
						return err
					}
					name, ok := key.(string)
					name = strings.ToLower(name)
					if !ok || keys[name] {
						return errors.New("chave JSON repetida ou inválida")
					}
					keys[name] = true
				}
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
			_, err = tokens.Token()
			return err
		}
		return nil
	}
	if err := visit(0); err != nil {
		return errors.New("JSON inválido ou ambíguo")
	}
	if _, err := tokens.Token(); err != io.EOF {
		return errors.New("JSON contém dados extras")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("JSON fora do esquema estrito")
	}
	return nil
}

func validate(fixtures Fixtures, rules Rules, fixturesHash string) error {
	if !fixtures.Synthetic || !rules.Synthetic || rules.FixturesSHA256 != fixturesHash {
		return errors.New("exigidos dados sintéticos e regras vinculadas ao hash dos casos")
	}
	if len(fixtures.Cases) == 0 || len(fixtures.Cases) > 100 || len(rules.Cases) != len(fixtures.Cases) {
		return errors.New("exigidos 1 a 100 casos, cada um com critérios externos")
	}
	ids, families := map[string]bool{}, map[string]string{}
	for _, item := range fixtures.Cases {
		date, err := time.Parse("2006-01-02", item.Today)
		if !identifier.MatchString(item.ID) || ids[item.ID] || !identifier.MatchString(item.Family) ||
			(item.Split != "valid" && item.Split != "test") || item.Ledger == nil ||
			err != nil || date.Format("2006-01-02") != item.Today ||
			(item.Agent != "otavio" && item.Agent != "lia" && item.Agent != "tito") ||
			strings.TrimSpace(item.Question) == "" || len(item.Question) > 4096 || len(item.History) > 12 {
			return errors.New("caso incompleto, repetido ou fora dos limites")
		}
		if item.Rates != nil {
			for _, period := range []struct{ value, layout string }{{item.Rates.Date, "2006-01-02"}, {item.Rates.IPCAMonth, "2006-01"}} {
				if period.value == "" {
					continue
				}
				parsed, err := time.Parse(period.layout, period.value)
				if err != nil || parsed.Format(period.layout) != period.value {
					return errors.New("data ou mês das taxas inválido")
				}
			}
		}
		if split := families[item.Family]; split != "" && split != item.Split {
			return errors.New("família cruza valid e test")
		}
		ids[item.ID], families[item.Family] = true, item.Split
		for _, message := range item.History {
			if (message.Role != "user" && message.Role != "assistant") || strings.TrimSpace(message.Content) == "" || len(message.Content) > 4096 {
				return errors.New("histórico inválido")
			}
		}
		ledger, _ := json.Marshal(item.Ledger)
		if len(ledger) > 64<<10 {
			return errors.New("livro excedeu 64 KiB")
		}
	}
	seen := map[string]bool{}
	for _, item := range rules.Cases {
		if !ids[item.ID] || seen[item.ID] || len(item.Criteria) == 0 || len(item.Criteria) > 20 {
			return errors.New("critérios ausentes, extras ou repetidos")
		}
		seen[item.ID] = true
		criteria := map[string]bool{}
		for _, rule := range item.Criteria {
			if !identifier.MatchString(rule.ID) || criteria[rule.ID] || !identifier.MatchString(rule.Category) ||
				!validCriterion(rule) {
				return errors.New("critério exige id, categoria e somente contains, forbids, calls ou proposals válido")
			}
			criteria[rule.ID] = true
		}
	}
	return nil
}
