package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/alexandreserra1/finan_rich/internal/store"
)

// syncPageSize limita quantos registros vão por resposta; o aparelho repete o
// pedido enquanto "more" for verdadeiro.
const syncPageSize = 500

type syncRequest struct {
	Cursor  int64          `json:"cursor"`  // última revisão que o aparelho já tem
	Changes []store.Change `json:"changes"` // o que o aparelho mudou desde a última sincronização
}

type syncResponse struct {
	Cursor  int64          `json:"cursor"`
	Changes []store.Change `json:"changes"`
	More    bool           `json:"more"`
	Notices []string       `json:"notices,omitempty"` // avisos (ex.: categoria em uso que não foi apagada)
}

// apiSync recebe as alterações do aparelho e devolve as que ele ainda não tem.
func (s *Server) apiSync(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if tooBig := new(http.MaxBytesError); errors.As(err, &tooBig) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "envio grande demais para uma vez só; atualize o app (ele manda em partes)")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "pedido de sincronização inválido")
		return
	}
	ctx := r.Context()
	var notices []string
	if len(req.Changes) > 0 {
		var err error
		if _, notices, err = s.store.ApplyChangesNotices(ctx, req.Changes); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, store.ErrInvalidChange) {
				status = http.StatusUnprocessableEntity
			}
			log.Printf("sync: %v", err)
			writeJSONError(w, status, err.Error())
			return
		}
	}
	changes, cursor, err := s.store.ChangesSince(ctx, req.Cursor, syncPageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if changes == nil {
		changes = []store.Change{}
	}
	writeJSON(w, http.StatusOK, syncResponse{Cursor: cursor, Changes: changes, More: len(changes) == syncPageSize, Notices: notices})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
