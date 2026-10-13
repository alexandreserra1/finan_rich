// Package wasmapi é a ponte entre o navegador e as regras em Go.
//
// Cada função recebe e devolve texto (JSON), o formato mais simples de passar
// pela fronteira entre JavaScript e WebAssembly. O cmd/finanwasm só registra
// estas funções no navegador; a lógica fica aqui, testável sem navegador.
package wasmapi

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/finance"
	"github.com/alexandreserra1/finan_rich/internal/importer"
	"github.com/alexandreserra1/finan_rich/internal/money"
	"github.com/alexandreserra1/finan_rich/internal/report"
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

// Export gera um arquivo para baixar (planilha, CSV, PDF, backup ou nota).
// options é o report.Options em JSON; nowMs é a hora do aparelho. O arquivo
// volta em JSON, com o conteúdo em base64.
func Export(ledgerJSON, optionsJSON string, nowMs int64) string {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	var o report.Options
	if err := json.Unmarshal([]byte(optionsJSON), &o); err != nil {
		return reply(nil, err)
	}
	f, err := report.Export(l, o, time.UnixMilli(nowMs))
	return reply(f, err)
}

// ReadBackup confere um arquivo de backup e devolve os registros validados.
func ReadBackup(content string) string {
	return reply(report.ReadBackup([]byte(content)))
}

// ImportPreview lê um extrato (OFX ou CSV, em base64) e devolve as linhas
// prontas para revisar: categoria sugerida, duplicados e o que ignorar.
func ImportPreview(ledgerJSON, fileBase64, optionsJSON string) string {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	data, err := base64.StdEncoding.DecodeString(fileBase64)
	if err != nil {
		return reply(nil, err)
	}
	var o importer.Options
	if optionsJSON != "" {
		if err := json.Unmarshal([]byte(optionsJSON), &o); err != nil {
			return reply(nil, err)
		}
	}
	st, err := importer.Parse(data)
	if err != nil {
		return reply(nil, err)
	}
	return reply(importer.Build(l, st, o), nil)
}

// planInput lê o livro-caixa e a data de hoje do aparelho ("2026-10-15").
func planInput(ledgerJSON, today string) (domain.Ledger, time.Time, error) {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return l, time.Time{}, err
	}
	day, err := time.Parse(domain.DateLayout, today)
	return l, day, err
}

// Goals devolve as metas com o progresso.
func Goals(ledgerJSON, today string) string {
	l, day, err := planInput(ledgerJSON, today)
	if err != nil {
		return reply(nil, err)
	}
	return reply(finance.Goals(l, day), nil)
}

// RecurringDue devolve os lançamentos que as recorrências devem gerar.
func RecurringDue(ledgerJSON, today string) string {
	l, day, err := planInput(ledgerJSON, today)
	if err != nil {
		return reply(nil, err)
	}
	due := finance.RecurringDue(l, day)
	if due == nil {
		due = []domain.Transaction{}
	}
	return reply(due, nil)
}

// Alerts devolve os avisos do painel.
func Alerts(ledgerJSON, today string) string {
	l, day, err := planInput(ledgerJSON, today)
	if err != nil {
		return reply(nil, err)
	}
	return reply(finance.Alerts(l, day), nil)
}

// ImportPreviewText é a ImportPreview de uma fatura em PDF: o aparelho extrai
// o texto (pdf.js) e manda as linhas; today ("2026-10-15") ajuda a achar o ano.
func ImportPreviewText(ledgerJSON, text, optionsJSON, today string) string {
	var l domain.Ledger
	if err := json.Unmarshal([]byte(ledgerJSON), &l); err != nil {
		return reply(nil, err)
	}
	var o importer.Options
	if optionsJSON != "" {
		if err := json.Unmarshal([]byte(optionsJSON), &o); err != nil {
			return reply(nil, err)
		}
	}
	day, err := time.Parse(domain.DateLayout, today)
	if err != nil {
		day = time.Now()
	}
	st, err := importer.ParseText(text, day)
	if err != nil {
		return reply(nil, err)
	}
	return reply(importer.Build(l, st, o), nil)
}
