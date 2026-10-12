// Package wasmapi é a ponte entre o navegador e as regras em Go.
//
// Cada função recebe e devolve texto (JSON), o formato mais simples de passar
// pela fronteira entre JavaScript e WebAssembly. O cmd/finanwasm só registra
// estas funções no navegador; a lógica fica aqui, testável sem navegador.
package wasmapi

import (
	"encoding/json"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

// result é a resposta de toda função: um valor ou um erro.
type result struct {
	OK    any    `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
}

func reply(v any, err error) string {
	r := result{OK: v}
	if err != nil {
		r = result{Error: err.Error()}
	}
	b, _ := json.Marshal(r)
	return string(b)
}

// Dashboard monta o painel do mês ("2026-10") a partir do livro-caixa em JSON.
func Dashboard(ledgerJSON, month string) string {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	return reply(finance.Dashboard(l, domain.ParseMonth(month)), nil)
}

// Debts monta a tela de dívidas.
func Debts(ledgerJSON string) string {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	return reply(finance.Debts(l), nil)
}

// Validate confere um registro antes de salvar no aparelho; devolve o registro
// normalizado.
func Validate(typ, recordJSON string) string {
	_, data, err := domain.DecodeRecord(typ, []byte(recordJSON))
	if err != nil {
		return reply(nil, err)
	}
	return reply(json.RawMessage(data), nil)
}

// ParseMoney lê "1.234,56" e devolve centavos.
func ParseMoney(s string) string {
	cents, err := money.Parse(s)
	return reply(cents, err)
}

// FormatMoney devolve "R$ 1.234,56".
func FormatMoney(cents int64) string { return money.Format(cents) }

// MonthLabel devolve "outubro de 2026" para "2026-10".
func MonthLabel(month string) string { return domain.ParseMonth(month).Label() }
