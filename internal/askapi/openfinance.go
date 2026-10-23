package askapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/pluggy"
)

// OpenFinanceRequest vem do app: as credenciais da Pluggy (lidas do cofre do
// sistema na hora) e desde quando buscar.
type OpenFinanceRequest struct {
	Credentials pluggy.Credentials `json:"credentials"`
	From        string             `json:"from"` // AAAA-MM-DD
}

// OpenFinance busca contas e lançamentos na Pluggy, direto do celular.
func OpenFinance(ctx context.Context, reqJSON string) string {
	var req OpenFinanceRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return reply(nil, err)
	}
	if _, err := time.Parse("2006-01-02", req.From); err != nil {
		return reply(nil, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return reply(pluggy.Fetch(ctx, http.DefaultClient, pluggy.BaseURL, req.Credentials, req.From))
}
