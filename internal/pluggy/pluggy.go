// Package pluggy busca contas e lançamentos pelo Open Finance, usando a API da
// Pluggy com as credenciais da própria pessoa (conta grátis em pluggy.ai e os
// bancos conectados em meu.pluggy.ai). Não passa por servidor do finan_rich: roda
// no computador ou no celular de quem usa. O resultado é um extrato
// (importer.Statement) e segue a mesma revisão da importação de arquivo.
package pluggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/importer"
)

// BaseURL é a API da Pluggy.
const BaseURL = "https://api.pluggy.ai"

// Limites para uma resposta estranha não travar o aparelho.
const (
	maxBody  = 8 << 20 // por resposta
	maxPages = 50      // por conta (até ~25 mil lançamentos)
)

// Credentials são as da pessoa: Client ID e Secret da aplicação dela na
// Pluggy, e os Item IDs dos bancos conectados.
type Credentials struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	ItemIDs      []string `json:"itemIds"`
}

// Account é uma conta ou cartão com os lançamentos já no formato de extrato.
type Account struct {
	Name      string             `json:"name"`
	Statement importer.Statement `json:"statement"`
}

// Fetch busca, de cada banco conectado, as contas e os lançamentos desde from
// (AAAA-MM-DD). Só entram os lançamentos que já caíram (POSTED).
func Fetch(ctx context.Context, hc *http.Client, base string, c Credentials, from string) ([]Account, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" || len(c.ItemIDs) == 0 {
		return nil, errors.New("preencha o Client ID, o Client Secret e pelo menos um Item ID (Ajustes → Open Finance)")
	}
	api := &client{hc: hc, base: strings.TrimRight(base, "/")}
	var auth struct {
		APIKey string `json:"apiKey"`
	}
	body, _ := json.Marshal(map[string]string{"clientId": strings.TrimSpace(c.ClientID), "clientSecret": strings.TrimSpace(c.ClientSecret)})
	if err := api.do(ctx, http.MethodPost, "/auth", bytes.NewReader(body), &auth); err != nil {
		var se statusError
		if errors.As(err, &se) && (se == http.StatusUnauthorized || se == http.StatusForbidden) {
			return nil, errors.New("a Pluggy recusou o Client ID ou o Client Secret; confira em dashboard.pluggy.ai")
		}
		return nil, err
	}
	api.key = auth.APIKey
	var out []Account
	for _, item := range c.ItemIDs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		var accs struct {
			Results []struct {
				ID, Type, Name, MarketingName, Number string
			} `json:"results"`
		}
		if err := api.do(ctx, http.MethodGet, "/accounts?"+url.Values{"itemId": {item}}.Encode(), nil, &accs); err != nil {
			return nil, fmt.Errorf("banco %s: %w", item, err)
		}
		for _, a := range accs.Results {
			st := importer.Statement{Format: "pluggy", Account: "pluggy:" + a.ID, Card: a.Type == "CREDIT"}
			rows, err := api.transactions(ctx, a.ID, from)
			if err != nil {
				return nil, fmt.Errorf("conta %s: %w", a.Name, err)
			}
			st.Rows = rows
			out = append(out, Account{Name: accountName(a.Name, a.MarketingName, a.Number), Statement: st})
		}
	}
	return out, nil
}

func (api *client) transactions(ctx context.Context, accountID, from string) ([]importer.Row, error) {
	var rows []importer.Row
	q := "?" + url.Values{"accountId": {accountID}, "dateFrom": {from}}.Encode()
	for page := 0; q != "" && page < maxPages; page++ {
		var res struct {
			Results []struct {
				ID, Description, DescriptionRaw, Date, Type, Status string
				Amount                                              float64
			} `json:"results"`
			Next *string `json:"next"`
		}
		if err := api.do(ctx, http.MethodGet, "/v2/transactions"+q, nil, &res); err != nil {
			return nil, err
		}
		for _, t := range res.Results {
			if t.Status != "POSTED" || len(t.Date) < 10 {
				continue
			}
			cents := int64(math.Round(math.Abs(t.Amount) * 100))
			if t.Type == "DEBIT" {
				cents = -cents
			}
			desc := strings.TrimSpace(t.Description)
			if desc == "" {
				desc = strings.TrimSpace(t.DescriptionRaw)
			}
			rows = append(rows, importer.Row{Date: t.Date[:10], Description: desc, AmountCents: cents, FITID: t.ID})
		}
		q = ""
		if res.Next != nil && strings.HasPrefix(*res.Next, "?") {
			q = *res.Next
		}
	}
	return rows, nil
}

// accountName: o nome comercial (ou o do banco) e o final do número.
func accountName(name, marketing, number string) string {
	if marketing != "" {
		name = marketing
	}
	if n := []rune(strings.TrimSpace(number)); len(n) >= 4 {
		name += " · final " + string(n[len(n)-4:])
	}
	return name
}

type client struct {
	hc        *http.Client
	base, key string
}

type statusError int

func (e statusError) Error() string { return fmt.Sprintf("a Pluggy respondeu %d", int(e)) }

func (api *client) do(ctx context.Context, method, path string, body io.Reader, into any) error {
	req, err := http.NewRequestWithContext(ctx, method, api.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if api.key != "" {
		req.Header.Set("X-API-KEY", api.key)
	}
	resp, err := api.hc.Do(req)
	if err != nil {
		return errors.New("não consegui falar com a Pluggy (sem internet?)") // o erro original pode ter a URL; nunca o segredo, mas também não ajuda
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusError(resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(into); err != nil {
		return errors.New("a Pluggy mandou uma resposta que não entendi")
	}
	return nil
}
