package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Banco Central falso: as três séries do SGS no formato de verdade.
func bcb(t *testing.T, series map[string]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for code, body := range series {
			if strings.Contains(r.URL.Path, "bcdata.sgs."+code+"/") {
				w.Write([]byte(body))
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch(t *testing.T) {
	srv := bcb(t, map[string]string{
		"432":   `[{"data":"09/12/2026","valor":"13.75"}]`,
		"4389":  `[{"data":"17/11/2026","valor":"13.65"}]`,
		"13522": `[{"data":"01/10/2026","valor":"4.6"}]`,
	})
	r, err := Fetch(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if r != (Rates{SelicBP: 1375, CDIBP: 1365, IPCABP: 460, IPCAMonth: "2026-10", Date: "2026-11-17"}) {
		t.Errorf("taxas = %+v", r)
	}
	want := "Taxas do Banco Central (17/11/2026): Selic 13,75% ao ano; CDI 13,65% ao ano; IPCA 4,60% em 12 meses (até outubro de 2026)."
	if got := r.Text(); got != want {
		t.Errorf("texto = %q", got)
	}
	if (Rates{}).Text() != "" {
		t.Error("sem taxas, sem texto")
	}
}

func TestFetchErrors(t *testing.T) {
	for name, series := range map[string]map[string]string{
		"fora do ar":    {},
		"valor errado":  {"432": `[{"data":"09/12/2026","valor":"abc"}]`, "4389": `[{"data":"17/11/2026","valor":"13.65"}]`, "13522": `[{"data":"01/10/2026","valor":"4.6"}]`},
		"lista vazia":   {"432": `[]`, "4389": `[]`, "13522": `[]`},
		"data estranha": {"432": `[{"data":"2026-12-09","valor":"13.75"}]`, "4389": `[{"data":"17/11/2026","valor":"13.65"}]`, "13522": `[{"data":"01/10/2026","valor":"4.6"}]`},
	} {
		srv := bcb(t, series)
		if r, err := Fetch(context.Background(), srv.Client(), srv.URL); err == nil {
			t.Errorf("%s: sem erro, %+v", name, r)
		}
	}
}

// O Banco Central às vezes responde 304, sem corpo, a um pedido sem condição (visto pelo curl e
// no app em 24/11/2026). A busca pede de novo com um parâmetro único no endereço.
func TestFetchRetries304(t *testing.T) {
	series := map[string]string{
		"432":   `[{"data":"09/12/2026","valor":"13.75"}]`,
		"4389":  `[{"data":"23/11/2026","valor":"13.65"}]`,
		"13522": `[{"data":"01/10/2026","valor":"4.63"}]`,
	}
	for name, misses := range map[string]int{"um 304": 1, "dois 304": 2} {
		seen := map[string]int{}
		var urls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			urls = append(urls, r.URL.String())
			for code, body := range series {
				if strings.Contains(r.URL.Path, "bcdata.sgs."+code+"/") {
					if seen[code]++; seen[code] <= misses {
						w.WriteHeader(http.StatusNotModified)
						return
					}
					w.Write([]byte(body))
					return
				}
			}
			http.NotFound(w, r)
		}))
		r, err := Fetch(context.Background(), srv.Client(), srv.URL)
		srv.Close()
		if err != nil || r.CDIBP != 1365 || r.IPCABP != 463 {
			t.Errorf("%s: %+v, %v", name, r, err)
		}
		if strings.Contains(urls[0], "_=") || !strings.Contains(urls[1], "&_=") || urls[1] == urls[2] {
			t.Errorf("%s: o 1º pedido vai como sempre; os de novo, cada um com um parâmetro único: %v", name, urls[:3])
		}
	}
	// Sempre 304: desiste depois de 3 pedidos, com o erro de sempre.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusNotModified) }))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL); err == nil || !strings.Contains(err.Error(), "resposta 304") || calls != 3 {
		t.Errorf("sempre 304: %v, %d pedidos", err, calls)
	}
}

func TestParseBP(t *testing.T) {
	for in, want := range map[string]int{"13.75": 1375, "13.7": 1370, "13": 1300, "4.635": 464, "0.004": 0, "-0.21": -21, "10,50": 1050} {
		if got, err := parseBP(in); err != nil || got != want {
			t.Errorf("parseBP(%q) = %d, %v; quer %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "1.2.3", "1e3"} {
		if _, err := parseBP(in); err == nil {
			t.Errorf("parseBP(%q) sem erro", in)
		}
	}
}
