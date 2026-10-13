package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func meta(id string) Meta { return Meta{ID: id, UpdatedAt: 1} }

func TestValidate(t *testing.T) {
	good := Transaction{Meta: meta("t1"), Kind: KindSaida, Description: "Aluguel", AmountCents: 100, CategoryID: "c1", Date: "2026-10-05"}
	if err := good.Validate(); err != nil {
		t.Fatalf("válido: %v", err)
	}
	bad := []Transaction{
		{Meta: Meta{ID: "", UpdatedAt: 1}, Kind: KindSaida, Description: "x", CategoryID: "c", Date: "2026-10-05"},
		{Meta: meta("t"), Kind: "outro", Description: "x", CategoryID: "c", Date: "2026-10-05"},
		{Meta: meta("t"), Kind: KindSaida, Description: " ", CategoryID: "c", Date: "2026-10-05"},
		{Meta: meta("t"), Kind: KindSaida, Description: "x", CategoryID: "c", Date: "05/10/2026"},
		{Meta: meta("t"), Kind: KindSaida, Description: "x", AmountCents: -1, CategoryID: "c", Date: "2026-10-05"},
	}
	for i, tx := range bad {
		if tx.Validate() == nil {
			t.Errorf("caso %d deveria ser inválido", i)
		}
	}
	// Apagado só precisa de id e data de atualização.
	if err := (Transaction{Meta: Meta{ID: "t", UpdatedAt: 5, Deleted: true}}).Validate(); err != nil {
		t.Errorf("apagado: %v", err)
	}
	if (Category{Meta: meta("c"), Name: "X", Bucket: "outro"}).Validate() == nil {
		t.Error("bucket inválido aceito")
	}
	if (Debt{Meta: meta("d"), Name: "X", DueDay: 40}).Validate() == nil {
		t.Error("dia 40 aceito")
	}
	split, _ := json.Marshal(Split{BucketNecessidades: 60, BucketDesejos: 30, BucketInvestimentos: 20})
	if (Pref{Meta: meta(PrefSplit), Value: split}).Validate() == nil {
		t.Error("divisão somando 110% aceita")
	}
}

func TestLedger(t *testing.T) {
	split, _ := json.Marshal(Split{BucketNecessidades: 60, BucketDesejos: 20, BucketInvestimentos: 20})
	l := Ledger{
		Categories: []Category{{Meta: Meta{ID: "c1", Deleted: true}, Name: "Antiga"}},
		Transactions: []Transaction{
			{Meta: Meta{ID: "a", UpdatedAt: 1}, Date: "2026-10-01"},
			{Meta: Meta{ID: "b", UpdatedAt: 1, Deleted: true}, Date: "2026-10-09"},
			{Meta: Meta{ID: "c", UpdatedAt: 2}, Date: "2026-10-05"},
		},
		Debts: []Debt{{Meta: Meta{ID: "barata"}, MonthlyRateBP: 100}, {Meta: Meta{ID: "cara"}, MonthlyRateBP: 900}},
		Prefs: []Pref{{Meta: Meta{ID: PrefSplit, UpdatedAt: 1}, Value: split}},
	}
	txs := l.ActiveTransactions()
	if len(txs) != 2 || txs[0].ID != "c" {
		t.Errorf("ativos = %+v", txs)
	}
	if l.ActiveDebts()[0].ID != "cara" {
		t.Error("dívida mais cara primeiro")
	}
	if l.CategoryIndex()["c1"].Name != "Antiga" {
		t.Error("categoria apagada deve continuar resolvível")
	}
	if l.Split()[BucketNecessidades] != 60 || (Ledger{}).Split()[BucketNecessidades] != 50 {
		t.Error("split")
	}
}

func TestMonth(t *testing.T) {
	m := NewMonth(2026, time.February)
	if !m.Contains("2026-02-28") || m.Contains("2026-03-01") || m.Day(31).Day() != 28 || m.Label() != "fevereiro de 2026" {
		t.Error("mês")
	}
}

func TestDecodeRecord(t *testing.T) {
	raw := []byte(`{"id":"t1","updatedAt":5,"kind":"saida","description":"x","amountCents":1,"categoryId":"c","date":"2026-01-02","extra":"<b>"}`)
	meta, data, err := DecodeRecord(TypeTransaction, raw)
	if err != nil || meta.ID != "t1" || meta.UpdatedAt != 5 || string(data) == string(raw) {
		t.Fatalf("decode: %+v %s %v", meta, data, err)
	}
	if _, _, err := DecodeRecord(TypeTransaction, []byte(`{"id":"t1","updatedAt":5}`)); err == nil {
		t.Error("lançamento sem campos deveria falhar")
	}
	if _, _, err := DecodeRecord("senha", raw); err == nil {
		t.Error("tipo desconhecido deveria falhar")
	}
}

func TestNotes(t *testing.T) {
	meta := func(id string, at int64) Meta { return Meta{ID: id, UpdatedAt: at} }
	if err := (Note{Meta: meta("a", 1), Body: "só texto"}).Validate(); err != nil {
		t.Errorf("nota só com texto: %v", err)
	}
	if err := (Note{Meta: meta("a", 1), Title: "  "}).Validate(); err == nil {
		t.Error("nota vazia deveria falhar")
	}
	if err := (Note{Meta: meta("a", 1), Title: strings.Repeat("é", NoteTitleMax+1)}).Validate(); err == nil {
		t.Error("título longo demais deveria falhar")
	}
	l := Ledger{Notes: []Note{
		{Meta: meta("velha", 1), Title: "velha"},
		{Meta: meta("nova", 3), Title: "nova"},
		{Meta: meta("fixada", 2), Title: "fixada", Pinned: true},
		{Meta: Meta{ID: "apagada", UpdatedAt: 9, Deleted: true}},
	}}
	var got []string
	for _, n := range l.ActiveNotes() {
		got = append(got, n.ID)
	}
	if strings.Join(got, ",") != "fixada,nova,velha" {
		t.Errorf("ordem das notas = %v", got)
	}
}
