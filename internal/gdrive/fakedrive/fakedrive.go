// Package fakedrive é um Google Drive falso para os testes do cliente (internal/gdrive)
// e do programa (internal/web). Ele guarda na memória só a pasta de dados do app
// (appDataFolder), nas mesmas rotas da API v3 que o site e o programa usam.
package fakedrive

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Drive é o servidor falso. Token, se preenchido, é o único Bearer aceito.
type Drive struct {
	*httptest.Server
	mu     sync.Mutex
	token  string
	files  map[string]*file
	order  []string
	writes int
	fail   func(*http.Request) (int, string)
}

type file struct {
	name    string
	version int
	content []byte
}

// New sobe o Drive falso; quem chama fecha com Close.
func New() *Drive {
	d := &Drive{files: map[string]*file{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /drive/v3/files", d.list)
	mux.HandleFunc("GET /drive/v3/files/{id}", d.download)
	mux.HandleFunc("POST /upload/drive/v3/files", d.create)
	mux.HandleFunc("PATCH /upload/drive/v3/files/{id}", d.update)
	d.Server = httptest.NewServer(d.check(mux))
	return d
}

// RequireToken faz o Drive aceitar só este token de acesso (os outros recebem 401).
func (d *Drive) RequireToken(token string) { d.mu.Lock(); d.token = token; d.mu.Unlock() }

// Fail faz o Drive responder status e body (o JSON de erro do Google) enquanto f
// devolver status maior que zero; nil volta ao normal.
func (d *Drive) Fail(f func(*http.Request) (status int, body string)) {
	d.mu.Lock()
	d.fail = f
	d.mu.Unlock()
}

// Put grava o arquivo como outro aparelho faria: cria se não existe, senão troca o
// conteúdo e sobe a versão. Não conta em Writes.
func (d *Drive) Put(name string, content []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range d.order {
		if f := d.files[id]; f.name == name {
			f.content, f.version = content, f.version+1
			return
		}
	}
	d.add(name, content)
}

// Content devolve o conteúdo do primeiro arquivo com esse nome (nil se não existe).
func (d *Drive) Content(name string) []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range d.order {
		if f := d.files[id]; f.name == name {
			return f.content
		}
	}
	return nil
}

// Writes conta as gravações feitas pela API (criar ou regravar).
func (d *Drive) Writes() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writes
}

func (d *Drive) add(name string, content []byte) string {
	id := fmt.Sprintf("arquivo-%d", len(d.order)+1)
	d.files[id] = &file{name: name, version: 1, content: content}
	d.order = append(d.order, id)
	return id
}

func (d *Drive) check(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		fail, token := d.fail, d.token
		d.mu.Unlock()
		if fail != nil {
			if status, body := fail(r); status > 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				io.WriteString(w, body)
				return
			}
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			writeError(w, http.StatusUnauthorized, "authError")
			return
		}
		next.ServeHTTP(w, r)
	})
}

var nameQuery = regexp.MustCompile(`^name = '([^']*)'$`)

func (d *Drive) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	m := nameQuery.FindStringSubmatch(q.Get("q"))
	if q.Get("spaces") != "appDataFolder" || m == nil {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	type item struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}
	d.mu.Lock()
	out := []item{}
	for _, id := range d.order {
		if f := d.files[id]; f.name == m[1] {
			out = append(out, item{id, strconv.Itoa(f.version)})
		}
	}
	d.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{"files": out})
}

func (d *Drive) download(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("alt") != "media" {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	d.mu.Lock()
	f := d.files[r.PathValue("id")]
	var content []byte
	if f != nil {
		content = f.content
	}
	d.mu.Unlock()
	if f == nil {
		writeError(w, http.StatusNotFound, "notFound")
		return
	}
	w.Write(content)
}

// create recebe o multipart/related da API: os metadados (nome e pasta) e o conteúdo.
func (d *Drive) create(w http.ResponseWriter, r *http.Request) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/related" || r.URL.Query().Get("uploadType") != "multipart" {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var meta struct {
		Name    string   `json:"name"`
		Parents []string `json:"parents"`
	}
	part, err := mr.NextPart()
	if err != nil || json.NewDecoder(part).Decode(&meta) != nil || len(meta.Parents) != 1 || meta.Parents[0] != "appDataFolder" {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	if part, err = mr.NextPart(); err != nil {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	content, _ := io.ReadAll(part)
	d.mu.Lock()
	id := d.add(meta.Name, content)
	d.writes++
	d.mu.Unlock()
	saved(w, id, 1)
}

func (d *Drive) update(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("uploadType") != "media" {
		writeError(w, http.StatusBadRequest, "badRequest")
		return
	}
	content, _ := io.ReadAll(r.Body)
	id := r.PathValue("id")
	d.mu.Lock()
	f := d.files[id]
	version := 0
	if f != nil {
		f.content, f.version = content, f.version+1
		version = f.version
		d.writes++
	}
	d.mu.Unlock()
	if f == nil {
		writeError(w, http.StatusNotFound, "notFound")
		return
	}
	saved(w, id, version)
}

func saved(w http.ResponseWriter, id string, version int) {
	json.NewEncoder(w).Encode(map[string]string{"id": id, "version": strconv.Itoa(version), "modifiedTime": time.Now().UTC().Format(time.RFC3339Nano)})
}

func writeError(w http.ResponseWriter, status int, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%d,"errors":[{"reason":%q}]}}`, status, reason)
}
