package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListModelsSkipsSmallContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("pedido = %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		// O Groq diz o tamanho do contexto; outros provedores não dizem (fica).
		fmt.Fprint(w, `{"data":[{"id":"grande","context_window":131072},{"id":"pequeno","context_window":4096},{"id":"sem-tamanho"}]}`)
	}))
	defer srv.Close()
	ids, err := ListModels(context.Background(), srv.URL, "k")
	if err != nil || strings.Join(ids, ",") != "grande,sem-tamanho" {
		t.Errorf("ListModels = %v, %v", ids, err)
	}
}
