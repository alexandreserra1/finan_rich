package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func change(t *testing.T, typ string, v any) Change {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return Change{Type: typ, Record: b}
}

func txRecord(id string, updated int64, desc string) domain.Transaction {
	return domain.Transaction{Meta: domain.Meta{ID: id, UpdatedAt: updated}, Kind: domain.KindSaida,
		Description: desc, AmountCents: 100, CategoryID: "cat-3", Date: "2026-10-05"}
}

func TestFreshInstallHasDefaults(t *testing.T) {
	s := newTestStore(t)
	l, err := s.Ledger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Categories) != 15 || l.Split()[domain.BucketNecessidades] != 50 {
		t.Errorf("instalação nova: %d categorias, split %v", len(l.Categories), l.Split())
	}
	if err := s.migrate(context.Background()); err != nil {
		t.Errorf("migrations devem ser idempotentes: %v", err)
	}
}

func TestApplyChangesLastWriteWins(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	start, _ := s.CurrentRev(ctx)

	n, err := s.ApplyChanges(ctx, []Change{change(t, TypeTransaction, txRecord("t1", 100, "celular"))})
	if err != nil || n != 1 {
		t.Fatalf("criar: n=%d err=%v", n, err)
	}
	// Uma edição mais antiga (feita offline em outro aparelho) perde.
	if n, _ := s.ApplyChanges(ctx, []Change{change(t, TypeTransaction, txRecord("t1", 50, "computador"))}); n != 0 {
		t.Error("alteração mais antiga não pode sobrescrever")
	}
	// A mais nova vence; reenviar a mesma é ignorado (idempotente).
	newer := change(t, TypeTransaction, txRecord("t1", 200, "computador"))
	if n, _ := s.ApplyChanges(ctx, []Change{newer, newer}); n != 1 {
		t.Errorf("mais nova: n=%d", n)
	}
	// Apagar também sincroniza.
	del := domain.Transaction{Meta: domain.Meta{ID: "t1", UpdatedAt: 300, Deleted: true}}
	if n, err := s.ApplyChanges(ctx, []Change{change(t, TypeTransaction, del)}); n != 1 || err != nil {
		t.Errorf("apagar: n=%d err=%v", n, err)
	}

	changes, cursor, err := s.ChangesSince(ctx, start, 100)
	if err != nil || len(changes) != 1 || cursor != start+3 {
		t.Fatalf("mudanças desde %d: %d itens, cursor %d, err %v", start, len(changes), cursor, err)
	}
	var got domain.Transaction
	json.Unmarshal(changes[0].Record, &got)
	if !got.Deleted || got.UpdatedAt != 300 {
		t.Errorf("registro final = %+v", got)
	}
	if l, _ := s.Ledger(ctx); len(l.ActiveTransactions()) != 0 {
		t.Error("lançamento apagado não pode aparecer como ativo")
	}
}

func TestApplyChangesValidates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bad := txRecord("t1", 1, "")
	cases := []Change{
		change(t, TypeTransaction, bad),
		change(t, "senha", map[string]string{"id": "x"}),
		{Type: TypeDebt, Record: json.RawMessage(`{"id":`)},
	}
	for i, c := range cases {
		if _, err := s.ApplyChanges(ctx, []Change{change(t, TypeTransaction, txRecord("ok", 1, "ok")), c}); err == nil {
			t.Errorf("caso %d deveria ser recusado", i)
		}
	}
	// Um lote com erro não grava nada.
	if l, _ := s.Ledger(ctx); len(l.Transactions) != 0 {
		t.Error("lote inválido gravou parte dos registros")
	}
	// Campos desconhecidos são descartados.
	extra := Change{Type: TypeTransaction, Record: json.RawMessage(`{"id":"x","updatedAt":1,"kind":"saida","description":"d","amountCents":1,"categoryId":"c","date":"2026-01-01","hack":"<script>"}`)}
	s.ApplyChanges(ctx, []Change{extra})
	changes, _, _ := s.ChangesSince(ctx, 1, 10)
	if len(changes) != 1 || string(changes[0].Record) == string(extra.Record) || json.Valid(changes[0].Record) == false {
		t.Errorf("registro normalizado = %s", changes[0].Record)
	}
}

func TestChangesSincePaginates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	var batch []Change
	for i := range 5 {
		batch = append(batch, change(t, TypeTransaction, txRecord(string(rune('a'+i)), 1, "x")))
	}
	s.ApplyChanges(ctx, batch)
	rev, _ := s.CurrentRev(ctx)
	first, cursor, _ := s.ChangesSince(ctx, rev-5, 3)
	rest, cursor2, _ := s.ChangesSince(ctx, cursor, 3)
	if len(first) != 3 || len(rest) != 2 || cursor2 != rev {
		t.Errorf("paginação: %d + %d, cursor final %d (rev %d)", len(first), len(rest), cursor2, rev)
	}
}

// TestMigrateFromV1 simula um banco da versão 1.0 com dados e confere que a
// conversão para registros sincronizáveis não perde nada.
func TestMigrateFromV1(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	for _, name := range []string{"migrations/001_init.sql", "migrations/002_sessions.sql"} {
		script, _ := migrationsFS.ReadFile(name)
		if _, err := db.Exec(string(script)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		db.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name)
	}
	mustExec := func(q string, args ...any) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`UPDATE settings SET value = '60' WHERE key = 'pct_necessidades'`)
	mustExec(`UPDATE settings SET value = '20' WHERE key = 'pct_desejos'`)
	mustExec(`INSERT INTO settings (key, value) VALUES ('ai_model', 'qwen3.5:9b')`)
	mustExec(`INSERT INTO transactions (kind, description, amount_cents, category_id, occurred_on) VALUES ('saida', 'Aluguel', 180000, 3, '2026-09-10')`)
	mustExec(`INSERT INTO debts (name, creditor, balance_cents, monthly_rate_bp, min_payment_cents, due_day) VALUES ('Cartão', 'Banco', 320000, 1400, 32000, 10)`)
	mustExec(`INSERT INTO debts (name, balance_cents) VALUES ('Sem dia', 1000)`)
	db.Close()

	s, err := Open(ctx, path) // aplica a 003
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	l, err := s.Ledger(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txs := l.ActiveTransactions()
	if len(txs) != 1 || txs[0].Description != "Aluguel" || txs[0].AmountCents != 180000 || txs[0].Date != "2026-09-10" {
		t.Fatalf("lançamentos migrados = %+v", txs)
	}
	if cat := l.CategoryIndex()[txs[0].CategoryID]; cat.Name != "Moradia" {
		t.Errorf("categoria do lançamento = %+v", cat)
	}
	debts := l.ActiveDebts()
	if len(debts) != 2 || debts[0].Name != "Cartão" || debts[0].DueDay != 10 || debts[0].MonthlyRateBP != 1400 || debts[1].DueDay != 0 {
		t.Errorf("dívidas migradas = %+v", debts)
	}
	if split := l.Split(); split[domain.BucketNecessidades] != 60 || split[domain.BucketDesejos] != 20 {
		t.Errorf("divisão migrada = %v", split)
	}
	for _, c := range mustChanges(t, s) {
		if _, _, err := decodeChange(c); err != nil {
			t.Errorf("registro migrado inválido: %v", err)
		}
	}
	if ai, _ := s.Settings(ctx, "ai_"); ai["model"] != "qwen3.5:9b" {
		t.Error("configurações da IA devem continuar")
	}
	if pct, _ := s.Settings(ctx, "pct_"); len(pct) != 0 {
		t.Error("percentuais antigos devem sair de settings")
	}
}

func mustChanges(t *testing.T, s *Store) []Change {
	changes, _, err := s.ChangesSince(context.Background(), 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

func TestChat(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.AppendChatTurn(ctx, "oi", "olá!", nil); err != nil {
		t.Fatal(err)
	}
	msgs, _ := s.ListChat(ctx)
	if len(msgs) != 2 || msgs[0].Role != RoleUser || msgs[1].Content != "olá!" {
		t.Errorf("chat = %+v", msgs)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.SaveSettings(ctx, "ai_", map[string]string{"model": "qwen3:4b"})
	s.SaveSettings(ctx, "ai_", map[string]string{"model": "qwen3.5:9b", "provider": "ollama"})
	got, err := s.Settings(ctx, "ai_")
	if err != nil || got["model"] != "qwen3.5:9b" || got["provider"] != "ollama" || len(got) != 2 {
		t.Errorf("settings = %v, err = %v", got, err)
	}
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	s.CreateSession(ctx, "valida", now.Add(time.Hour))
	s.CreateSession(ctx, "vencida", now.Add(-time.Hour))
	for token, want := range map[string]bool{"valida": true, "vencida": false, "inexistente": false} {
		if ok, err := s.SessionValid(ctx, token, now); err != nil || ok != want {
			t.Errorf("SessionValid(%s) = %v, %v", token, ok, err)
		}
	}
	s.DeleteSession(ctx, "valida")
	if ok, _ := s.SessionValid(ctx, "valida", now); ok {
		t.Error("sessão apagada continua válida")
	}
}

func TestDevices(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := s.CreateDevice(ctx, "Celular", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.DeviceByToken(ctx, "outro", time.Now()); ok {
		t.Fatal("token desconhecido aceito")
	}
	d, ok, err := s.DeviceByToken(ctx, "hash1", time.Now())
	if err != nil || !ok || d.ID != id || d.Name != "Celular" {
		t.Fatalf("DeviceByToken = %+v %v %v", d, ok, err)
	}
	list, err := s.Devices(ctx)
	if err != nil || len(list) != 1 || list[0].LastSeen.IsZero() || list[0].CreatedAt.IsZero() {
		t.Fatalf("Devices = %+v %v", list, err)
	}
	if err := s.DeleteDevice(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.DeviceByToken(ctx, "hash1", time.Now()); ok {
		t.Fatal("aparelho removido continua aceito")
	}
}

func TestNotesSync(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	note := domain.Note{Meta: domain.Meta{ID: "n1", UpdatedAt: 10}, Title: "Meta", Body: "Juntar R$ 5.000 até dezembro", Pinned: true}
	if n, err := s.ApplyChanges(ctx, []Change{change(t, TypeNote, note)}); n != 1 || err != nil {
		t.Fatalf("nota: n=%d err=%v", n, err)
	}
	empty := domain.Note{Meta: domain.Meta{ID: "n2", UpdatedAt: 10}}
	if _, err := s.ApplyChanges(ctx, []Change{change(t, TypeNote, empty)}); err == nil {
		t.Error("nota vazia deveria ser recusada")
	}
	l, err := s.Ledger(ctx)
	if err != nil || len(l.ActiveNotes()) != 1 || l.ActiveNotes()[0].Body != note.Body || !l.ActiveNotes()[0].Pinned {
		t.Fatalf("Ledger.Notes = %+v (%v)", l.Notes, err)
	}
}

func TestAccountsSync(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	card := domain.Account{Meta: domain.Meta{ID: "c1", UpdatedAt: 10}, Name: "Cartão Nubank", Kind: domain.AccountCartao, ClosingDay: 3, DueDay: 10}
	tx := txRecord("t1", 10, "Geladeira")
	tx.AccountID = "c1"
	if n, err := s.ApplyChanges(ctx, []Change{change(t, TypeAccount, card), change(t, TypeTransaction, tx)}); n != 2 || err != nil {
		t.Fatalf("conta: n=%d err=%v", n, err)
	}
	l, err := s.Ledger(ctx)
	if err != nil || len(l.ActiveAccounts()) != 1 || l.Accounts[0].ClosingDay != 3 || l.Transactions[0].AccountID != "c1" {
		t.Fatalf("Ledger = %+v %+v (%v)", l.Accounts, l.Transactions, err)
	}
}

func TestDefaultCategoriesMatchComputer(t *testing.T) {
	s := newTestStore(t)
	l, err := s.Ledger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]string{}
	for _, c := range l.Categories {
		have[c.Name] = c.Bucket
	}
	for _, c := range domain.DefaultCategories(1) {
		if have[c.Name] != c.Bucket {
			t.Errorf("categoria padrão %q (%s) não existe igual no computador", c.Name, c.Bucket)
		}
	}
}

// Celular usado sozinho e depois pareado: as categorias padrão dele se juntam
// às do computador, sem duplicar, e os lançamentos passam para as do computador.
func TestPhoneDefaultsMergeIntoComputer(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	l, _ := s.Ledger(ctx)
	mercado := ""
	for _, c := range l.Categories {
		if c.Name == "Mercado" {
			mercado = c.ID
		}
	}
	rec := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	phone := []Change{
		// O lançamento vem antes da categoria: a ordem do envio não importa.
		{TypeTransaction, rec(domain.Transaction{Meta: domain.Meta{ID: "t1", UpdatedAt: 100}, Kind: domain.KindSaida, Description: "Feira", AmountCents: 5000, CategoryID: "padrao-mercado", Date: "2026-10-01"})},
		{TypeGoal, rec(domain.Goal{Meta: domain.Meta{ID: "g1", UpdatedAt: 100}, Name: "Reserva", TargetCents: 100000, CategoryID: "padrao-mercado", StartDate: "2026-10-01"})},
	}
	for _, c := range domain.DefaultCategories(50) {
		phone = append(phone, Change{TypeCategory, rec(c)})
	}
	// Categoria criada no celular, sem par no computador: fica.
	phone = append(phone, Change{TypeCategory, rec(domain.Category{Meta: domain.Meta{ID: "pet", UpdatedAt: 50}, Name: "Pet", Bucket: domain.BucketDesejos})})
	if _, err := s.ApplyChanges(ctx, phone); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Ledger(ctx)
	active := 0
	for _, c := range l.Categories {
		if !c.Deleted {
			active++
		}
	}
	if active != len(domain.DefaultCategories(0))+1 {
		t.Errorf("categorias ativas = %d; as padrão do celular deveriam se juntar às do computador", active)
	}
	for _, tx := range l.Transactions {
		if tx.ID == "t1" && (tx.CategoryID != mercado || tx.UpdatedAt != 101) {
			t.Errorf("lançamento = %+v; quer categoria %s e updatedAt 101 (volta ao celular)", tx, mercado)
		}
	}
	if l.Goals[0].CategoryID != mercado {
		t.Errorf("meta = %+v", l.Goals[0])
	}
	// Um lançamento novo do celular, mais tarde, com a categoria antiga: também vai para a do computador.
	s.ApplyChanges(ctx, []Change{{TypeTransaction, rec(domain.Transaction{Meta: domain.Meta{ID: "t2", UpdatedAt: 200}, Kind: domain.KindSaida, Description: "Pão", AmountCents: 800, CategoryID: "padrao-mercado", Date: "2026-10-02"})}})
	l, _ = s.Ledger(ctx)
	for _, tx := range l.Transactions {
		if tx.ID == "t2" && tx.CategoryID != mercado {
			t.Errorf("lançamento tardio = %+v", tx)
		}
	}
}
