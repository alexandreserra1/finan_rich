package store

import (
	"context"
	"strings"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func pet(updated int64, deleted bool) domain.Category {
	return domain.Category{Meta: domain.Meta{ID: "pet", UpdatedAt: updated, Deleted: deleted}, Name: "Pet", Bucket: domain.BucketDesejos}
}

func petTx(id string, updated int64, category string, deleted bool) domain.Transaction {
	tx := txRecord(id, updated, "Ração")
	tx.CategoryID, tx.Deleted = category, deleted
	return tx
}

// storedCategory devolve a categoria como está no servidor.
func storedCategory(t *testing.T, s *Store, id string) domain.Category {
	t.Helper()
	l, err := s.Ledger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return l.CategoryIndex()[id]
}

// Um aparelho apaga a categoria (sem conexão) enquanto outro já lançou nela:
// o apagamento é recusado com aviso, a categoria volta mais nova que o
// apagamento e o resto do envio é gravado.
func TestDeleteCategoryInUseIsRefused(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.ApplyChanges(ctx, []Change{change(t, TypeCategory, pet(10, false)), change(t, TypeTransaction, petTx("t1", 10, "pet", false))}); err != nil {
		t.Fatal(err)
	}
	rec := domain.Recurring{Meta: domain.Meta{ID: "r1", UpdatedAt: 10}, Kind: domain.KindSaida, Description: "Plano pet", AmountCents: 5000, CategoryID: "pet", Day: 5, Start: "2026-10"}
	if _, err := s.ApplyChanges(ctx, []Change{change(t, TypeRecurring, rec)}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.CurrentRev(ctx)

	note := domain.Note{Meta: domain.Meta{ID: "n1", UpdatedAt: 30}, Title: "Ideia"}
	n, notices, err := s.ApplyChangesNotices(ctx, []Change{change(t, TypeCategory, pet(20, true)), change(t, TypeNote, note)})
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "não foi possível apagar: a categoria Pet ainda tem 1 lançamento e 1 recorrente") {
		t.Errorf("avisos = %q", notices)
	}
	if c := storedCategory(t, s, "pet"); c.Deleted || c.UpdatedAt != 21 || c.Name != "Pet" {
		t.Errorf("categoria = %+v; quer ativa, updatedAt 21 (vence o apagamento no aparelho)", c)
	}
	changes, _, _ := s.ChangesSince(ctx, before, 100)
	var sawPet, sawNote bool
	for _, c := range changes {
		sawPet = sawPet || c.Type == TypeCategory && !strings.Contains(string(c.Record), "deleted")
		sawNote = sawNote || c.Type == TypeNote
	}
	if !sawPet || !sawNote {
		t.Errorf("o aparelho precisa receber a categoria de volta e a nota: %s", changes)
	}
}

// Ordem inversa: o apagamento chega primeiro (ninguém usava), depois um
// lançamento de outro aparelho nela. A categoria volta, sem aviso.
func TestCategoryComesBackWhenUsedAfterDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.ApplyChanges(ctx, []Change{change(t, TypeCategory, pet(10, false))})
	if _, notices, err := s.ApplyChangesNotices(ctx, []Change{change(t, TypeCategory, pet(20, true))}); err != nil || len(notices) != 0 {
		t.Fatalf("apagar sem uso: avisos=%q err=%v", notices, err)
	}
	if !storedCategory(t, s, "pet").Deleted {
		t.Fatal("categoria sem uso deveria ser apagada")
	}
	_, notices, err := s.ApplyChangesNotices(ctx, []Change{change(t, TypeTransaction, petTx("t1", 15, "pet", false))})
	if err != nil || len(notices) != 0 {
		t.Fatalf("avisos=%q err=%v", notices, err)
	}
	if c := storedCategory(t, s, "pet"); c.Deleted || c.UpdatedAt != 21 {
		t.Errorf("categoria = %+v; deveria voltar (o lançamento não fica sem categoria)", c)
	}
	// Uma meta também segura a categoria.
	s.ApplyChanges(ctx, []Change{change(t, TypeTransaction, petTx("t1", 30, "pet", true)), change(t, TypeCategory, pet(40, true))})
	goal := domain.Goal{Meta: domain.Meta{ID: "g1", UpdatedAt: 50}, Name: "Castração", TargetCents: 50000, CategoryID: "pet", StartDate: "2026-10-01"}
	s.ApplyChanges(ctx, []Change{change(t, TypeGoal, goal)})
	if c := storedCategory(t, s, "pet"); c.Deleted {
		t.Errorf("meta nova na categoria apagada: categoria = %+v", c)
	}
}

// Mover os lançamentos e apagar a categoria no mesmo envio pode (a fila manda
// a categoria antes dos lançamentos); lançamento apagado não segura.
func TestDeleteCategoryAfterMovingItsItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	s.ApplyChanges(ctx, []Change{change(t, TypeCategory, pet(10, false)), change(t, TypeTransaction, petTx("t1", 10, "pet", false)), change(t, TypeTransaction, petTx("t2", 10, "pet", false))})
	_, notices, err := s.ApplyChangesNotices(ctx, []Change{
		change(t, TypeCategory, pet(20, true)),
		change(t, TypeTransaction, petTx("t1", 20, "cat-3", false)),
		change(t, TypeTransaction, petTx("t2", 20, "pet", true)),
	})
	if err != nil || len(notices) != 0 {
		t.Fatalf("avisos=%q err=%v", notices, err)
	}
	if c := storedCategory(t, s, "pet"); !c.Deleted || c.UpdatedAt != 20 {
		t.Errorf("categoria = %+v; deveria estar apagada", c)
	}
}
