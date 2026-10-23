package web

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/pluggy"
)

// Open Finance pelo Meu Pluggy, no computador: as credenciais da pessoa ficam
// na tabela settings (não vão para o celular nem para o backup em JSON; o .db
// completo leva, como leva a chave da IA) e a
// busca vai daqui direto para a Pluggy, sem passar por servidor do finan_rich.
const pluggyPrefix = "pluggy_"

var itemID = regexp.MustCompile(`^[0-9A-Za-z-]{1,64}$`)

func (s *Server) pluggyCredentials(ctx context.Context) (pluggy.Credentials, error) {
	m, err := s.store.Settings(ctx, pluggyPrefix)
	if err != nil {
		return pluggy.Credentials{}, err
	}
	return pluggy.Credentials{ClientID: m["client_id"], ClientSecret: m["client_secret"], ItemIDs: strings.Fields(m["items"])}, nil
}

// saveOpenFinance grava as credenciais. Segredo em branco mantém o atual (a
// tela nunca mostra o segredo salvo).
func (s *Server) saveOpenFinance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items := strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(r.FormValue("items")))
	for _, it := range items {
		if !itemID.MatchString(it) {
			redirect(w, r, "/configuracoes#open-finance", "", errors.New("Item ID inválido: copie do painel da Pluggy (Ir para Demo → Copiar Item ID)"))
			return
		}
	}
	values := map[string]string{"client_id": strings.TrimSpace(r.FormValue("client_id")), "items": strings.Join(items, " ")}
	if secret := strings.TrimSpace(r.FormValue("client_secret")); secret != "" {
		values["client_secret"] = secret
	}
	if r.FormValue("acao") == "apagar" {
		values = map[string]string{"client_id": "", "client_secret": "", "items": ""}
	}
	err := s.store.SaveSettings(ctx, pluggyPrefix, values)
	redirect(w, r, "/configuracoes#open-finance", "Open Finance salvo.", err)
}

// apiOpenFinance busca as contas e os lançamentos desde "from" (AAAA-MM-DD).
func (s *Server) apiOpenFinance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "pedido inválido")
		return
	}
	if _, err := time.Parse("2006-01-02", req.From); err != nil {
		writeJSONError(w, http.StatusBadRequest, "data inicial inválida")
		return
	}
	c, err := s.pluggyCredentials(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	if c.ClientID == "" || c.ClientSecret == "" || len(c.ItemIDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "configure o Open Finance em Ajustes (Client ID, Client Secret e Item ID da Pluggy)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	accs, err := pluggy.Fetch(ctx, http.DefaultClient, cmp.Or(os.Getenv("FINAN_PLUGGY_URL"), pluggy.BaseURL), c, req.From)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error()) // mensagens do pacote pluggy, feitas para a pessoa e sem segredo
		return
	}
	writeJSON(w, http.StatusOK, accs)
}
