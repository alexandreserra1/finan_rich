package site

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	files = fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"privacidade.html": {Data: []byte("política")},
		"static/app.js":    {Data: []byte("console.log(1)")},
	}
	t.Cleanup(func() { files = nil })
	h := Handler()
	get := func(path, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for path, want := range map[string]string{"/": "<html>app</html>", "/lancamentos": "<html>app</html>", "/privacidade.html": "política", "/privacidade": "política", "/static/app.js": "console.log(1)"} {
		if rec := get(path, ""); rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("%s = %d %q", path, rec.Code, rec.Body.String())
		}
	}
	if rec := get("/static/sumiu.js", ""); rec.Code != http.StatusNotFound {
		t.Errorf("arquivo que não existe = %d, quer 404 (não a página do app)", rec.Code)
	}
	first := get("/static/app.js", "")
	if first.Header().Get("X-Frame-Options") != "DENY" || first.Header().Get("ETag") == "" {
		t.Errorf("cabeçalhos = %v", first.Header())
	}
	if rec := get("/static/app.js", first.Header().Get("ETag")); rec.Code != http.StatusNotModified {
		t.Errorf("arquivo igual = %d, quer 304 (não baixar de novo)", rec.Code)
	}
	if rec := get("/../../etc/passwd", ""); rec.Code < 400 {
		t.Errorf("fora da pasta = %d, quer recusa", rec.Code)
	}
}

func TestHandlerWithoutBundle(t *testing.T) {
	if Handler() != nil {
		t.Error("sem a etiqueta sitebundle o programa não serve site")
	}
}

func TestDownload(t *testing.T) {
	h := Download()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://rich.test"+path, nil))
		return rec
	}
	if rec := get("/baixar/android"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sem o bucket configurado = %d, quer 503 (não um link quebrado)", rec.Code)
	}
	apkBase = "https://finan-rich-app.pages.dev"
	t.Cleanup(func() { apkBase = "" })
	for path, want := range map[string]string{"/baixar/android": "https://finan-rich-app.pages.dev/android/finan_rich.apk", "/baixar/versao": "https://finan-rich-app.pages.dev/android/latest.json"} {
		if rec := get(path); rec.Code != http.StatusFound || rec.Header().Get("Location") != want {
			t.Errorf("%s = %d %q, quer %q", path, rec.Code, rec.Header().Get("Location"), want)
		}
	}
	if rec := get("/baixar/qr.png"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(rec.Body.String(), "\x89PNG") {
		t.Errorf("QR code = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
