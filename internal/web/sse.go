package web

import (
	"encoding/json"
	"net/http"
)

// sseWriter envia eventos Server-Sent Events no formato "data: {json}\n\n".
// É usado pelo chat e pelo download de modelos.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSE(w http.ResponseWriter) (*sseWriter, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	return &sseWriter{w: w, flusher: flusher}, true
}

func (s *sseWriter) Send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.w.Write([]byte("data: "))
	s.w.Write(b)
	s.w.Write([]byte("\n\n"))
	s.flusher.Flush()
}

func (s *sseWriter) Error(err error) { s.Send(map[string]string{"error": err.Error()}) }
func (s *sseWriter) Done()           { s.Send(map[string]bool{"done": true}) }
