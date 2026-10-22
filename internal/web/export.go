package web

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Arquivos de static/ que só servem ao app no navegador (PWA).
var browserOnly = map[string]bool{"sw.js": true, "manifest.webmanifest": true, "offline.html": true}

// appCSP é a política de segurança das telas do app do celular (vai numa tag
// <meta>, porque lá não há servidor para mandar o cabeçalho). Igual à
// browserCSP, mas a rede fica liberada: o app fala com o computador pareado
// (http:// no Wi-Fi) e com as IAs na nuvem (https://).
const appCSP = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; style-src-attr 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self' https: http:; object-src 'none'; base-uri 'none'; form-action 'self'"

// ExportApp grava numa pasta as telas do app que funcionam sem servidor
// (index.html + static/), para empacotar no app do celular (Capacitor).
// O app monta as telas com os dados do aparelho e sincroniza com o
// computador pareado.
func ExportApp(dir string) error {
	s, err := New(nil, Options{})
	if err != nil {
		return err
	}
	var page bytes.Buffer
	if err := s.pages["app"].Execute(&page, map[string]any{"Page": "dashboard", "AppPage": "dashboard", "Mobile": true}); err != nil {
		return err
	}
	// A política vem logo depois do charset, antes de qualquer script ou estilo.
	const charset = `<meta charset="utf-8">`
	if !bytes.Contains(page.Bytes(), []byte(charset)) {
		return errors.New("index.html sem <meta charset>: não há onde pôr a política de segurança")
	}
	csp := charset + "\n" + `<meta http-equiv="Content-Security-Policy" content="` + appCSP + `">`
	index := bytes.Replace(page.Bytes(), []byte(charset), []byte(csp), 1)
	if err := os.RemoveAll(filepath.Join(dir, "static")); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), index, 0o644); err != nil {
		return err
	}
	return fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || browserOnly[d.Name()] {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
