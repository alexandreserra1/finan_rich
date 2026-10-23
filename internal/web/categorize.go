package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/importer"
	"github.com/alexandreserra1/finan_rich/internal/llm"
)

const maxCategorize = 300

type categorizeRequest struct {
	Items []advisor.CategorizeItem `json:"items"`
}

type categorizeResponse struct {
	Categories []string `json:"categories"` // ID da categoria de cada item ("" = sem sugestão)
}

// apiCategorize sugere categorias com a IA para lançamentos importados de um
// extrato. Só recebe as descrições (e se é entrada ou saída), não os valores.
func (s *Server) apiCategorize(w http.ResponseWriter, r *http.Request) {
	var req categorizeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "pedido inválido")
		return
	}
	if len(req.Items) == 0 || len(req.Items) > maxCategorize {
		writeJSONError(w, http.StatusBadRequest, "envie de 1 a 300 lançamentos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	provider, ai, err := s.provider(ctx)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	l, err := s.store.Ledger(ctx)
	if err != nil {
		internalError(w, err)
		return
	}
	if !llm.IsLocal(ai.Config) {
		// IA na nuvem: vai só a "assinatura" da descrição, sem números (CPF,
		// conta, código de autorização) nem "PIX ENVIADO" e afins.
		for i := range req.Items {
			req.Items[i].Description = importer.DescKey(req.Items[i].Description)
		}
	}
	cats, err := advisor.Categorize(ctx, provider, l.Categories, req.Items)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		writeJSONError(w, status, "a IA não respondeu: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, categorizeResponse{Categories: cats})
}
