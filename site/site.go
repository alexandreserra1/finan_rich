// Package site serve o site do finan_rich: as telas do app (geradas por
// "go run ./tools/mobilebuild -site site/www") junto do intermediário da IA,
// no mesmo programa da Vercel (cmd/api). As telas entram no programa só com
// a etiqueta de build sitebundle (bundle.go); sem ela, Handler devolve nil.
package site

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// files são as telas (index.html, privacidade.html, static/...).
var files fs.FS

// Handler devolve o servidor do site, ou nil se o programa foi compilado sem as telas.
func Handler() http.Handler {
	if files == nil {
		return nil
	}
	tags := etags(files)
	server := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Strict-Transport-Security", "max-age=31536000")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, ok := tags[name]; !ok {
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html" // rotas das telas (/lancamentos, /dividas...) abrem o app
		}
		// Sem data nos arquivos embutidos: a impressão digital evita baixar de
		// novo os .wasm (megabytes) a cada visita, e muda quando o site muda.
		h.Set("ETag", tags[name])
		h.Set("Cache-Control", "no-cache")
		if name == "index.html" {
			http.ServeFileFS(w, r, files, name)
			return
		}
		server.ServeHTTP(w, r)
	})
}

func etags(fsys fs.FS) map[string]string {
	out := map[string]string{}
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[p] = `"` + hex.EncodeToString(sum[:8]) + `"`
		return nil
	})
	return out
}
