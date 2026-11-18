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
