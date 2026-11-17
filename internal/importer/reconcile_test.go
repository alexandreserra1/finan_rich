package importer

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func TestReconcileImportedSources(t *testing.T) {
	row := Row{Date: "2026-10-05", Description: "Pão de Açúcar", AmountCents: -2590, FITID: "outro-banco"}
	base := domain.Transaction{Meta: domain.Meta{ID: "imp-notificacao", UpdatedAt: 1}, Date: row.Date,
		Kind: domain.KindSaida, AmountCents: 2590, Description: "PAO DE ACUCAR"}
	for _, format := range []string{"notificacao", "ofx", "csv", "pdf"} {
		t.Run(format, func(t *testing.T) {
			for _, test := range []struct {
				name        string
				description string
				duplicate   bool
			}{
				{"descrição normalizada igual", "PAO DE ACUCAR", true},
				{"descrição semelhante", "Compra no débito - Pão de Açúcar 1234", true},
				{"mesmo valor e dia não bastam", "Farmácia Central", false},
				{"descrição vazia não basta", "", false},
			} {
				t.Run(test.name, func(t *testing.T) {
					existing := base
					existing.Description = test.description
					ledger := domain.Ledger{Transactions: []domain.Transaction{existing}}
					before := append([]domain.Transaction(nil), ledger.Transactions...)
					preview := Build(ledger, Statement{Format: format, Rows: []Row{row}}, Options{})
					want := StatusNew
					if test.duplicate {
						want = StatusDuplicate
					}
					if item := preview.Items[0]; item.Status != want || item.Selected == test.duplicate {
						t.Errorf("conciliação = %+v; esperado status=%s", item, want)
					}
					if !reflect.DeepEqual(before, ledger.Transactions) {
						t.Fatal("a revisão alterou os lançamentos existentes")
					}
				})
			}
		})
	}
}

func TestReconcileAccountAndBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, account, destination, date, kind string
		amount                                 int64
		deleted, duplicate                     bool
	}{
		{name: "ambos sem conta", duplicate: true},
		{name: "mesma conta", account: "conta-a", destination: "conta-a", duplicate: true},
		{name: "contas diferentes", account: "conta-a", destination: "conta-b"},
		{name: "destino desconhecido", account: "conta-a"},
		{name: "origem desconhecida", destination: "conta-a"},
		{name: "outro dia", date: "2026-10-06"},
		{name: "outro sentido", kind: domain.KindEntrada},
		{name: "outros centavos", amount: 501},
		{name: "excluído", deleted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			existing := domain.Transaction{Meta: domain.Meta{ID: "imp-origem", Deleted: test.deleted},
				Date: "2026-10-05", Kind: domain.KindSaida, Description: "Café", AmountCents: 500, AccountID: test.account}
			if test.date != "" {
				existing.Date = test.date
			}
			if test.kind != "" {
				existing.Kind = test.kind
			}
			if test.amount != 0 {
				existing.AmountCents = test.amount
			}
			optionsJSON, err := json.Marshal(map[string]string{"accountId": test.destination})
			if err != nil {
				t.Fatal(err)
			}
			var options Options
			if err := json.Unmarshal(optionsJSON, &options); err != nil {
				t.Fatal(err)
			}
			preview := Build(domain.Ledger{Transactions: []domain.Transaction{existing}}, Statement{Account: "número-ofx",
				Rows: []Row{{Date: "2026-10-05", Description: "Café", AmountCents: -500}}}, options)
			want := StatusNew
			if test.duplicate {
				want = StatusDuplicate
			}
			if item := preview.Items[0]; item.Status != want || item.Selected == test.duplicate {
				t.Errorf("conciliação = %+v; esperado status=%s", item, want)
			}
		})
	}
}

func TestReconcileConsumesMatchesAndReservesImportedIDs(t *testing.T) {
	row := Row{Date: "2026-10-05", Description: "Café", AmountCents: -500, FITID: "compra-1"}
	second := row
	second.FITID = "compra-2"
	statement := Statement{Format: "ofx", Rows: []Row{row, second}}
	for _, count := range []int{1, 2} {
		ledger := domain.Ledger{}
		for _, id := range []string{"imp-notificacao-1", "imp-notificacao-2"}[:count] {
			ledger.Transactions = append(ledger.Transactions, domain.Transaction{Meta: domain.Meta{ID: id},
				Date: row.Date, Description: row.Description, AmountCents: 500, Kind: domain.KindSaida})
		}
		preview := Build(ledger, statement, Options{})
		for index, item := range preview.Items {
			want := StatusNew
			if index < count {
				want = StatusDuplicate
			}
			if item.Status != want || item.Selected != (want == StatusNew) {
				t.Errorf("%d origens, linha %d = %+v; esperado %s", count, index, item, want)
			}
		}
	}
	importedID := Build(domain.Ledger{}, statement, Options{}).Items[0].ID
	ledger := domain.Ledger{Transactions: []domain.Transaction{{Meta: domain.Meta{ID: importedID},
		Date: row.Date, Description: row.Description, AmountCents: 500, Kind: domain.KindSaida}}}
	for _, rows := range [][]Row{{row, second}, {second, row}} {
		statement.Rows = rows
		for _, item := range Build(ledger, statement, Options{}).Items {
			want := StatusNew
			if item.ID == importedID {
				want = StatusImported
			}
			if item.Status != want || item.Selected != (want == StatusNew) {
				t.Errorf("reserva do ID existente = %+v; esperado %s", item, want)
			}
		}
	}
}

func TestReconcileOptionsAndReview(t *testing.T) {
	var options Options
	if err := json.Unmarshal([]byte(`{"accountId":"conta-a","documentKind":"fatura","requireReview":true}`), &options); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["accountId"] != "conta-a" || fields["documentKind"] != "fatura" || fields["requireReview"] != true {
		t.Errorf("opções não preservadas: %s", encoded)
	}
	statement := Statement{Rows: []Row{{Date: "2026-10-05", Description: "Café", AmountCents: -500}}}
	before := Build(domain.Ledger{}, statement, Options{AccountID: options.AccountID}).Items[0]
	after := Build(domain.Ledger{}, statement, options).Items[0]
	if !before.Selected || after.Selected || after.Status != StatusNew || after.ID != before.ID {
		t.Errorf("revisão obrigatória: antes=%+v depois=%+v", before, after)
	}
}

func TestReconcilePreservesManualMatching(t *testing.T) {
	ledger := domain.Ledger{Transactions: []domain.Transaction{{Meta: domain.Meta{ID: "manual"}, Date: "2026-10-05",
		Description: "Outra descrição", Kind: domain.KindSaida, AmountCents: 500}}}
	row := Row{Date: "2026-10-05", Description: "Café", AmountCents: -500}
	preview := Build(ledger, Statement{Rows: []Row{row, row}}, Options{})
	if preview.Items[0].Status != StatusDuplicate || preview.Items[0].Selected || preview.Items[1].Status != StatusNew || !preview.Items[1].Selected {
		t.Errorf("conciliação manual alterada: %+v", preview.Items)
	}
}

func TestReconcilePreservesLegacyIDs(t *testing.T) {
	row := Row{Date: "2026-10-05", Description: "Café", AmountCents: -500}
	statement := Statement{Rows: []Row{row, row}}
	preview := Build(domain.Ledger{}, statement, Options{})
	for index, want := range []string{"imp-ea78da65363861954929b2a5", "imp-a6149e7b4345ce96e49bc489"} {
		if preview.Items[index].ID != want {
			t.Errorf("ID legado da linha %d = %s; esperado %s", index, preview.Items[index].ID, want)
		}
	}
	row.FITID = "compra-1"
	statement = Statement{Account: "conta-ofx", Rows: []Row{row}}
	if item := Build(domain.Ledger{}, statement, Options{}).Items[0]; item.ID != "imp-60bf44ef523c7e61634c0d3a" {
		t.Errorf("ID legado OFX alterado: %s", item.ID)
	}
}

func TestReconcileNamespacesNewIDsByAccount(t *testing.T) {
	row := Row{Date: "2026-10-05", Description: "Café", AmountCents: -500}
	statement := Statement{Format: "csv", Rows: []Row{row, row}}
	legacy := Build(domain.Ledger{}, statement, Options{})
	first := Build(domain.Ledger{}, statement, Options{AccountID: "conta-a"})
	ledger := domain.Ledger{}
	for index, item := range first.Items {
		if item.ID == legacy.Items[index].ID {
			t.Errorf("linha %d sem namespace da conta: %s", index, item.ID)
		}
		ledger.Transactions = append(ledger.Transactions, domain.Transaction{Meta: domain.Meta{ID: item.ID},
			Date: item.Date, Description: item.Description, Kind: item.Kind, AmountCents: item.AmountCents, AccountID: "conta-a"})
	}
	if first.Items[0].ID == first.Items[1].ID {
		t.Fatal("duas compras iguais precisam de IDs distintos")
	}
	for _, account := range []string{"conta-a", "conta-b"} {
		preview := Build(ledger, statement, Options{AccountID: account})
		for index, item := range preview.Items {
			if account == "conta-a" {
				if item.ID != first.Items[index].ID || item.Status != StatusImported || item.Selected {
					t.Errorf("reimportação na mesma conta: %+v", item)
				}
			} else if item.ID == first.Items[index].ID || item.Status != StatusNew || !item.Selected {
				t.Errorf("compra legítima em outra conta bloqueada: %+v", item)
			}
		}
	}
}

func TestReconcileLegacyIDRespectsAccountIncludingDeleted(t *testing.T) {
	statement := Statement{Format: "csv", Rows: []Row{{Date: "2026-10-05", Description: "Café", AmountCents: -500}}}
	legacy := Build(domain.Ledger{}, statement, Options{}).Items[0]
	for _, deleted := range []bool{false, true} {
		ledger := domain.Ledger{Transactions: []domain.Transaction{{Meta: domain.Meta{ID: legacy.ID, Deleted: deleted},
			Date: legacy.Date, Description: legacy.Description, Kind: legacy.Kind, AmountCents: legacy.AmountCents, AccountID: "conta-a"}}}
		for _, account := range []string{"conta-a", "conta-b"} {
			item := Build(ledger, statement, Options{AccountID: account}).Items[0]
			if account == "conta-a" {
				if item.ID != legacy.ID || item.Status != StatusImported || item.Selected {
					t.Errorf("ID legado da mesma conta, excluído=%v: %+v", deleted, item)
				}
			} else if item.ID == legacy.ID || item.Status != StatusNew || !item.Selected {
				t.Errorf("ID legado de outra conta, excluído=%v: %+v", deleted, item)
			}
		}
	}
}

func TestReconcileManualMatchingRespectsDestinationAccount(t *testing.T) {
	statement := Statement{Rows: []Row{{Date: "2026-10-05", Description: "Café", AmountCents: -500}}}
	for _, test := range []struct {
		name, account, destination string
		duplicate                  bool
	}{
		{"mesma conta", "conta-a", "conta-a", true},
		{"outra conta", "conta-a", "conta-b", false},
		{"manual sem conta", "", "conta-a", false},
		{"destino vazio preserva comportamento antigo", "conta-a", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger := domain.Ledger{Transactions: []domain.Transaction{{Meta: domain.Meta{ID: "manual"},
				Date: "2026-10-05", Description: "Outra descrição", Kind: domain.KindSaida, AmountCents: 500, AccountID: test.account}}}
			item := Build(ledger, statement, Options{AccountID: test.destination}).Items[0]
			want := StatusNew
			if test.duplicate {
				want = StatusDuplicate
			}
			if item.Status != want || item.Selected == test.duplicate {
				t.Errorf("conciliação manual = %+v; esperado %s", item, want)
			}
		})
	}
}
