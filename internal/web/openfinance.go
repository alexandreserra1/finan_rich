package web

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
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

func configured(c pluggy.Credentials) bool {
	return c.ClientID != "" && c.ClientSecret != "" && len(c.ItemIDs) > 0
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
// "auto" é a busca automática da tela, que aceita o que o computador já buscou.
func (s *Server) apiOpenFinance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		Auto bool   `json:"auto"`
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
	if !configured(c) {
		writeJSONError(w, http.StatusBadRequest, "configure o Open Finance em Ajustes (Client ID, Client Secret e Item ID da Pluggy)")
		return
	}
	maxAge := time.Duration(0)
	if req.Auto {
		maxAge = bankEvery
	}
	accs, err := s.fetchBank(r.Context(), c, req.From, maxAge)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error()) // mensagens do pacote pluggy, feitas para a pessoa e sem segredo
		return
	}
	writeJSON(w, http.StatusOK, accs)
}

// Busca em segundo plano: enquanto o programa do computador estiver aberto, ele
// busca no banco a cada 6 horas, mesmo com o navegador fechado, e guarda o
// resultado na memória. A busca automática das telas (deste computador e do
// celular pareado) recebe o guardado e avisa no sininho na hora; "Buscar do
// banco" vai sempre à Pluggy. Nada entra sem revisão (openfinance.js).
const bankEvery = 6 * time.Hour // a Pluggy atualiza com o banco algumas vezes por dia

type bankCache struct {
	mu   sync.Mutex // uma busca de cada vez: quem chega no meio espera e usa o resultado
	key  string     // data inicial e credenciais da busca guardada
	at   time.Time
	accs []pluggy.Account
}

// fetchBank busca desde from, ou devolve a busca guardada da mesma data e das
// mesmas credenciais se ela tiver menos de maxAge.
func (s *Server) fetchBank(ctx context.Context, c pluggy.Credentials, from string, maxAge time.Duration) ([]pluggy.Account, error) {
	b := &s.bank
	b.mu.Lock()
	defer b.mu.Unlock()
	key := strings.Join(append([]string{from, c.ClientID, c.ClientSecret}, c.ItemIDs...), "\n")
	if b.key == key && time.Since(b.at) < maxAge {
		return b.accs, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	accs, err := pluggy.Fetch(ctx, http.DefaultClient, cmp.Or(os.Getenv("FINAN_PLUGGY_URL"), pluggy.BaseURL), c, from)
	if err != nil {
		return nil, err
	}
	b.key, b.at, b.accs = key, time.Now(), accs
	return accs, nil
}

// WatchOpenFinance busca no banco a cada 6 horas até ctx acabar. Sem Open
// Finance configurado, não faz nada; sem conexão, tenta de novo meia hora depois.
func (s *Server) WatchOpenFinance(ctx context.Context) {
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for {
		s.refreshBank(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) refreshBank(ctx context.Context) {
	c, err := s.pluggyCredentials(ctx)
	if err != nil || !configured(c) {
		return
	}
	if _, err := s.fetchBank(ctx, c, bankFrom(time.Now()), bankEvery); err != nil && ctx.Err() == nil {
		log.Printf("Open Finance: %v", err) // mensagens do pacote pluggy, sem segredo
	}
}

// bankFrom é a data inicial da busca em segundo plano: o dia 1º do mês passado,
// a mesma do defaultFrom de openfinance.js (senão a tela não usa o guardado).
func bankFrom(now time.Time) string {
	return time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Format(time.DateOnly)
}
