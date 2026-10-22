package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Apagar uma categoria que outro aparelho usou: a sincronização dá certo, o
// aparelho recebe a categoria de volta e um aviso para mostrar.
func TestSyncCategoryInUseNotice(t *testing.T) {
	a := newApp(t, fakeOllama(t).URL)
	tx := domain.Transaction{Meta: domain.Meta{ID: "t1", UpdatedAt: 1000}, Kind: domain.KindSaida,
		Description: "Mercado", AmountCents: 9000, CategoryID: "cat-5", Date: "2026-10-12"}
	if code, _ := a.postJSON("/api/sync", map[string]any{"cursor": 0, "changes": []any{map[string]any{"type": "transaction", "record": tx}}}); code != http.StatusOK {
		t.Fatalf("lançamento: %d", code)
	}
	deletedAt := time.Now().UnixMilli() + 1000 // mais novo que as categorias da instalação
	cat := domain.Category{Meta: domain.Meta{ID: "cat-5", UpdatedAt: deletedAt, Deleted: true}, Name: "Mercado", Bucket: domain.BucketNecessidades}
	code, body := a.postJSON("/api/sync", map[string]any{"cursor": 0, "changes": []any{map[string]any{"type": "category", "record": cat}}})
	notices, _ := body["notices"].([]any)
	if code != http.StatusOK || len(notices) != 1 || !strings.Contains(notices[0].(string), "Mercado ainda tem 1 lançamento") {
		t.Fatalf("apagar em uso: %d %v", code, body)
	}
	back := false
	for _, c := range body["changes"].([]any) {
		r := c.(map[string]any)["record"].(map[string]any)
		back = back || r["id"] == "cat-5" && r["deleted"] == nil && r["updatedAt"] == float64(deletedAt+1)
	}
	if !back {
		t.Errorf("a categoria deveria voltar ao aparelho: %v", body["changes"])
	}
}
