package wasmapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, s string) (map[string]any, string) {
	t.Helper()
	var r struct {
		OK    json.RawMessage `json:"ok"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("resposta não é JSON: %s", s)
	}
	var v map[string]any
	json.Unmarshal(r.OK, &v)
	return v, r.Error
}

const ledger = `{
  "categories": [{"id":"sal","updatedAt":1,"name":"Salário","bucket":"renda"},{"id":"laz","updatedAt":1,"name":"Lazer","bucket":"desejos"}],
  "transactions": [
    {"id":"a","updatedAt":1,"kind":"entrada","description":"Salário","amountCents":500000,"categoryId":"sal","date":"2026-10-05"},
    {"id":"b","updatedAt":1,"kind":"saida","description":"Bar","amountCents":200000,"categoryId":"laz","date":"2026-10-06"}
  ],
  "debts": [{"id":"d","updatedAt":1,"name":"Cartão","balanceCents":320000,"monthlyRateBp":1400,"minPaymentCents":32000}],
  "prefs": []
}`

func TestDashboard(t *testing.T) {
	v, errMsg := decode(t, Dashboard(ledger, "2026-10"))
	if errMsg != "" || v["balanceCents"] != float64(300000) || v["label"] != "outubro de 2026" {
		t.Fatalf("painel = %v %s", v, errMsg)
	}
	if _, errMsg := decode(t, Dashboard("{", "2026-10")); errMsg == "" {
		t.Error("JSON inválido deveria dar erro")
	}
}

func TestDebts(t *testing.T) {
	v, _ := decode(t, Debts(ledger))
	if v["monthlyInterestCents"] != float64(44800) || len(v["debts"].([]any)) != 1 {
		t.Errorf("dívidas = %v", v)
	}
}

func TestValidateAndMoney(t *testing.T) {
	_, errMsg := decode(t, Validate("transaction", `{"id":"x","updatedAt":1,"kind":"saida","description":"","amountCents":1,"categoryId":"c","date":"2026-10-01"}`))
	if !strings.Contains(errMsg, "descrição") {
		t.Errorf("validação = %q", errMsg)
	}
	if !strings.Contains(ParseMoney("1.234,56"), "123456") || !strings.Contains(ParseMoney("abc"), "error") {
		t.Error("parseMoney")
	}
	if FormatMoney(123456) != "R$ 1.234,56" || MonthLabel("2026-02") != "fevereiro de 2026" {
		t.Error("formatação")
	}
}
