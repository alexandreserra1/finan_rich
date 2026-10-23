package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// O caminho chega em ?path= (vercel.json) e vai para as rotas do intermediário.
func TestHandlerRoutesRewrittenPath(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "teste")
	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/api/index?path=healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodPost, "/api/index?path=chat/completions", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pedido fora do formato do app = %d, quer 403", rec.Code)
	}
}
