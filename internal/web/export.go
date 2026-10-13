package web

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
)

// Arquivos de static/ que só servem ao app no navegador (PWA).
var browserOnly = map[string]bool{"sw.js": true, "manifest.webmanifest": true, "offline.html": true}

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
	if err := os.RemoveAll(filepath.Join(dir, "static")); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), page.Bytes(), 0o644); err != nil {
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
