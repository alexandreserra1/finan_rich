package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.7.0", "1.6.0", true}, {"1.10.0", "1.9.2", true}, {"2.0.0", "1.99.99", true},
		{"1.6.0", "1.6.0", false}, {"1.5.9", "1.6.0", false}, {"1.6.1", "1.6", true},
	} {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestUpdateChecker(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/alexandreserra1/finan_rich/releases/tag/v1.7.0", http.StatusFound)
	}))
	defer gh.Close()

	wait := func(u *updateChecker) (string, bool) {
		u.available() // a primeira chamada só dispara a consulta
		for i := 0; i < 100; i++ {
			u.mu.Lock()
			done := !u.checked.IsZero()
			u.mu.Unlock()
			if done {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		return u.available()
	}

	u := newUpdateChecker("v1.6.0")
	u.url = gh.URL
	if v, ok := wait(u); !ok || v != "1.7.0" {
		t.Errorf("1.6.0 rodando: available() = %q, %v", v, ok)
	}
	u = newUpdateChecker("1.7.0")
	u.url = gh.URL
	if _, ok := wait(u); ok {
		t.Error("já na última versão não deveria avisar")
	}
	if _, ok := newUpdateChecker("dev").available(); ok {
		t.Error("build de desenvolvimento não avisa")
	}
	// Sem internet: não avisa, nem trava.
	u = newUpdateChecker("1.6.0")
	u.url = "http://127.0.0.1:1"
	if _, ok := wait(u); ok {
		t.Error("sem rede não deveria avisar")
	}
}

func TestUpdateBannerOnlyOnThisComputer(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	a.srv.updates = &updateChecker{current: "1.6.0", latest: "1.7.0", checked: time.Now()}
	for _, path := range []string{"/", "/configuracoes"} {
		if body := a.do("GET", path, local, nil).Body.String(); !strings.Contains(body, "Saiu a versão 1.7.0") || !strings.Contains(body, "install.") {
			t.Errorf("%s no computador deveria avisar da versão nova", path)
		}
	}
	if body := a.do("GET", "/parear?codigo=X", phone, nil).Body.String(); strings.Contains(body, "Saiu a versão") {
		t.Error("o celular não deveria ver o aviso (atualiza pela loja)")
	}
}
