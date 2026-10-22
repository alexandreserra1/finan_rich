package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// staticFiles entrega /static/ com ETag (o navegador revalida em vez de
// baixar de novo) e compactado com gzip quando o navegador aceita: o
// finance.wasm cai de ~5 MB para ~1,5 MB na primeira abertura pelo celular.
// Os arquivos são embutidos e não mudam enquanto o programa roda, então cada
// um é lido e compactado uma vez só.
type staticFiles struct{ cache sync.Map } // caminho → *staticFile

type staticFile struct {
	raw, gz []byte
	etag    string
	ctype   string
}

var compressible = map[string]bool{".wasm": true, ".js": true, ".css": true, ".svg": true, ".html": true, ".json": true, ".webmanifest": true}

func (s *staticFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	f, err := s.load(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("ETag", f.etag)
	h.Set("Cache-Control", "no-cache") // sempre confere o ETag: arquivo novo a cada versão
	h.Set("Vary", "Accept-Encoding")
	if f.ctype != "" {
		h.Set("Content-Type", f.ctype)
	}
	if r.Header.Get("If-None-Match") == f.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := f.raw
	if f.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = f.gz
	}
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

func (s *staticFiles) load(name string) (*staticFile, error) {
	if f, ok := s.cache.Load(name); ok {
		return f.(*staticFile), nil
	}
	raw, err := staticFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	ext := path.Ext(name)
	f := &staticFile{raw: raw, etag: `"` + hex.EncodeToString(sum[:8]) + `"`, ctype: mime.TypeByExtension(ext)}
	if ext == ".wasm" {
		f.ctype = "application/wasm" // necessário para o navegador compilar enquanto baixa
	}
	if compressible[ext] && len(raw) > 1024 {
		var b bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		zw.Write(raw)
		zw.Close()
		f.gz = b.Bytes()
	}
	s.cache.Store(name, f)
	return f, nil
}
