package wasmapi

import (
	"encoding/base64"
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

func TestExport(t *testing.T) {
	v, errMsg := decode(t, Export(ledger, `{"format":"csv","scope":"month","month":"2026-10"}`, 1791900000000))
	if errMsg != "" || v["name"] != "finan_rich_lancamentos_2026-10.csv" || v["mime"] != "text/csv;charset=utf-8" {
		t.Fatalf("export csv: %v %s", v, errMsg)
	}
	data, _ := base64.StdEncoding.DecodeString(v["data"].(string))
	if !strings.Contains(string(data), "2026-10-06;Saída;Bar;Lazer;Desejos;-2000,00") {
		t.Errorf("conteúdo: %s", data)
	}
	if _, errMsg := decode(t, Export(ledger, `{"format":"doc"}`, 1)); errMsg == "" {
		t.Error("formato desconhecido deveria dar erro")
	}

	v, _ = decode(t, Export(ledger, `{"format":"json"}`, 1791900000000))
	backup, _ := base64.StdEncoding.DecodeString(v["data"].(string))
	var r struct {
		OK    []map[string]any `json:"ok"`
		Error string           `json:"error"`
	}
	json.Unmarshal([]byte(ReadBackup(string(backup))), &r)
	if r.Error != "" || len(r.OK) != 5 {
		t.Errorf("backup lido de volta: %d registros, erro %q", len(r.OK), r.Error)
	}
}

func TestImportPreview(t *testing.T) {
	csv := base64.StdEncoding.EncodeToString([]byte("Data,Valor,Descrição\n03/10/2026,-45.90,UBER *TRIP\n"))
	var r struct {
		OK struct {
			Items []struct {
				Kind        string `json:"kind"`
				AmountCents int64  `json:"amountCents"`
			} `json:"items"`
		} `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal([]byte(ImportPreview(ledger, csv, `{}`)), &r)
	if r.Error != "" || len(r.OK.Items) != 1 || r.OK.Items[0].Kind != "saida" || r.OK.Items[0].AmountCents != 4590 {
		t.Fatalf("prévia: %+v", r)
	}
	if _, errMsg := decode(t, ImportPreview(ledger, base64.StdEncoding.EncodeToString([]byte("nada")), "")); errMsg == "" {
		t.Error("arquivo inválido deveria dar erro")
	}
}

func TestPlans(t *testing.T) {
	withPlans := strings.Replace(ledger, `"prefs": []`, `"prefs": [],
  "goals": [{"id":"g","updatedAt":1,"name":"Reserva","targetCents":100000,"savedCents":0,"startDate":"2026-01-01"}],
  "recurring": [{"id":"r","updatedAt":1,"kind":"saida","description":"Aluguel","amountCents":150000,"categoryId":"laz","day":5,"start":"2026-10"}]`, 1)
	var goals struct{ OK []map[string]any }
	json.Unmarshal([]byte(Goals(withPlans, "2026-10-10")), &goals)
	if len(goals.OK) != 1 || goals.OK[0]["name"] != "Reserva" {
		t.Errorf("metas: %+v", goals)
	}
	var due struct{ OK []map[string]any }
	json.Unmarshal([]byte(RecurringDue(withPlans, "2026-10-10")), &due)
	if len(due.OK) != 1 || due.OK[0]["id"] != "rec-r-2026-10" {
		t.Errorf("recorrentes: %+v", due)
	}
	var alerts struct{ OK []map[string]any }
	json.Unmarshal([]byte(Alerts(withPlans, "2026-10-10")), &alerts)
	if len(alerts.OK) == 0 {
		t.Errorf("alertas: %+v", alerts)
	}
	if _, errMsg := decode(t, Goals(withPlans, "ontem")); errMsg == "" {
		t.Error("data inválida deveria dar erro")
	}
}

func TestCanDeleteCategory(t *testing.T) {
	if r := CanDeleteCategory(ledger, "laz"); !strings.Contains(r, `"error":"a categoria Lazer ainda tem 1 lançamento;`) {
		t.Errorf("em uso: %s", r)
	}
	if r := CanDeleteCategory(ledger, "outra"); r != `{"ok":true}` {
		t.Errorf("sem uso: %s", r)
	}
}

func TestSearchTransactions(t *testing.T) {
	v, errMsg := decode(t, SearchTransactions(ledger, `{"text":"bar","month":"2026-10"}`))
	if errMsg != "" || v["count"] != float64(1) || v["outCents"] != float64(200000) || v["inCents"] != float64(0) {
		t.Errorf("busca = %v %s", v, errMsg)
	}
	if _, errMsg := decode(t, SearchTransactions(ledger, "{")); errMsg == "" {
		t.Error("filtro inválido deveria dar erro")
	}
}

func TestAccountsAndInstallments(t *testing.T) {
	withCard := strings.Replace(ledger, `"prefs": []`, `"prefs": [],
  "accounts": [{"id":"c","updatedAt":1,"name":"Cartão","kind":"cartao","initialCents":0,"closingDay":3,"dueDay":10}]`, 1)
	var parts struct {
		OK    []map[string]any
		Error string
	}
	json.Unmarshal([]byte(Installments(`{"id":"g","kind":"saida","description":"Geladeira","amountCents":1000,"categoryId":"laz","date":"2026-10-02","accountId":"c"}`, 3)), &parts)
	if parts.Error != "" || len(parts.OK) != 3 || parts.OK[0]["amountCents"] != float64(334) || parts.OK[2]["description"] != "Geladeira (3/3)" {
		t.Fatalf("parcelas: %+v", parts)
	}
	if _, errMsg := decode(t, Installments(`{"id":"g","amountCents":1,"date":"2026-10-02"}`, 3)); errMsg == "" {
		t.Error("valor menor que as parcelas deveria dar erro")
	}
	v, errMsg := decode(t, Accounts(withCard, "2026-10-05"))
	if errMsg != "" || v["cardsOpenCents"] != float64(0) || len(v["accounts"].([]any)) != 1 {
		t.Errorf("contas: %v %s", v, errMsg)
	}
	var nw struct{ OK []map[string]any }
	json.Unmarshal([]byte(NetWorth(withCard, "2026-10-05")), &nw)
	if len(nw.OK) != 12 || nw.OK[11]["netCents"] != float64(-320000) {
		t.Errorf("patrimônio: %+v", nw)
	}
}
