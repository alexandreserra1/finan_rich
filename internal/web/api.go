package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

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
	Now     int64          `json:"now"`               // hora deste computador (ms): o aparelho acerta o carimbo das edições por ela
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
			if !errors.Is(err, store.ErrInvalidChange) {
				internalError(w, err)
				return
			}
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error()) // o que estava errado no registro
			return
		}
	}
	changes, cursor, err := s.store.ChangesSince(ctx, req.Cursor, syncPageSize)
	if err != nil {
		internalError(w, err)
		return
	}
	if changes == nil {
		changes = []store.Change{}
	}
	writeJSON(w, http.StatusOK, syncResponse{Cursor: cursor, Changes: changes, More: len(changes) == syncPageSize, Notices: notices, Now: time.Now().UnixMilli()})
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

// internalError grava o detalhe só no log: caminhos e erros do banco não vão
// para quem pediu (pode ser outro aparelho na rede).
func internalError(w http.ResponseWriter, err error) {
	log.Printf("erro: %v", err)
	writeJSONError(w, http.StatusInternalServerError, "erro interno no computador; os detalhes estão na janela do finan_rich")
}
