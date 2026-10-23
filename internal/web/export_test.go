package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Todo módulo JavaScript precisa estar no cache do service worker; senão o
// app no navegador não abre sem conexão (o import falha).
func TestServiceWorkerPrecachesEveryModule(t *testing.T) {
	sw, err := staticFS.ReadFile("static/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	fs.WalkDir(staticFS, "static/js", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.Contains(string(sw), `"/`+path+`"`) {
			t.Errorf("%s falta na lista PRECACHE do sw.js", path)
		}
		return err
	})
}

func TestExportApp(t *testing.T) {
	dir := t.TempDir()
	if err := ExportApp(dir, false); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(index)
	for _, want := range []string{`data-app="mobile"`, `id="app"`, "/static/js/main.js", `href="/configuracoes"`} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html sem %q", want)
		}
	}
	for _, unwanted := range []string{"manifest.webmanifest", "data-install", "/sair"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("index.html do app não deveria ter %q", unwanted)
		}
	}
	for _, f := range []string{"static/finance.wasm", "static/js/main.js", "static/js/pages/device.js", "static/app.css"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("faltou %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "sw.js")); err == nil {
		t.Error("sw.js não vai para o app")
	}
	if strings.Contains(page, "data-site") || !strings.Contains(page, "connect-src 'self' https: http:") {
		t.Error("o app do celular não é o site: alcança o computador por http no Wi-Fi")
	}
}

// O site (as mesmas telas na internet) se identifica e só fala https.
func TestExportSite(t *testing.T) {
	dir := t.TempDir()
	if err := ExportApp(dir, true); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(index)
	if !strings.Contains(page, `data-site="web"`) || !strings.Contains(page, "connect-src 'self' https:;") {
		t.Errorf("index.html do site:\n%s", page[:min(len(page), 600)])
	}
}
