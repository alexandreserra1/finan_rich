package pluggy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePluggy responde como a API da Pluggy: chave, contas do item e
// lançamentos em duas páginas (cursor "after").
func fakePluggy(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["clientId"] != "id" || body["clientSecret"] != "segredo" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"message":"invalid credentials"}`))
				return
			}
			w.Write([]byte(`{"apiKey":"chave"}`))
			return
		}
		if r.Header.Get("X-API-KEY") != "chave" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.URL.Path == "/accounts" && r.URL.Query().Get("itemId") == "item-1":
			w.Write([]byte(`{"results":[
				{"id":"conta-1","type":"BANK","name":"Conta corrente","marketingName":"","number":"0001/12345-6"},
				{"id":"cartao-1","type":"CREDIT","name":"Mastercard","marketingName":"Nubank Ultravioleta","number":"9876"}]}`))
		case r.URL.Path == "/v2/transactions" && r.URL.Query().Get("accountId") == "conta-1":
			if r.URL.Query().Get("dateFrom") != "2026-09-01" {
				t.Errorf("dateFrom = %q", r.URL.Query().Get("dateFrom"))
			}
			if r.URL.Query().Get("after") == "" {
				w.Write([]byte(`{"results":[
					{"id":"t1","description":"Salário","amount":5200,"date":"2026-10-05T00:00:00.000Z","type":"CREDIT","status":"POSTED"},
					{"id":"t2","description":"Mercado","amount":-123.45,"date":"2026-10-06T00:00:00.000Z","type":"DEBIT","status":"POSTED"}],
					"next":"?accountId=conta-1&dateFrom=2026-09-01&after=c2"}`))
				return
			}
			w.Write([]byte(`{"results":[
				{"id":"t3","description":"","descriptionRaw":"PIX ENVIADO JOAO","amount":-50,"date":"2026-10-07T00:00:00.000Z","type":"DEBIT","status":"POSTED"},
				{"id":"t4","description":"Ainda não caiu","amount":-9.9,"date":"2026-10-08T00:00:00.000Z","type":"DEBIT","status":"PENDING"}],"next":null}`))
		case r.URL.Path == "/v2/transactions" && r.URL.Query().Get("accountId") == "cartao-1":
			w.Write([]byte(`{"results":[{"id":"c1","description":"iFood","amount":74.9,"date":"2026-10-03T12:00:00.000Z","type":"DEBIT","status":"POSTED"}],"next":null}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestFetch(t *testing.T) {
	srv := fakePluggy(t)
	defer srv.Close()
	accs, err := Fetch(context.Background(), srv.Client(), srv.URL, Credentials{ClientID: "id", ClientSecret: "segredo", ItemIDs: []string{"item-1"}}, "2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(accs) != 2 || accs[0].Name != "Conta corrente · final 45-6" || accs[1].Name != "Nubank Ultravioleta · final 9876" {
		t.Fatalf("contas = %+v", accs)
	}
	conta, cartao := accs[0].Statement, accs[1].Statement
	if conta.Account != "pluggy:conta-1" || conta.Card || !cartao.Card {
		t.Errorf("conta = %q cartão=%v/%v", conta.Account, conta.Card, cartao.Card)
	}
	// Só o que já caiu (POSTED); sentido pelo "type", não pelo sinal.
	got := []string{}
	for _, r := range conta.Rows {
		got = append(got, fmt.Sprintf("%s|%s|%s|%d", r.Date, r.Description, r.FITID, r.AmountCents))
	}
	want := "2026-10-05|Salário|t1|520000 2026-10-06|Mercado|t2|-12345 2026-10-07|PIX ENVIADO JOAO|t3|-5000"
	if strings.Join(got, " ") != want {
		t.Errorf("lançamentos =\n%s\nwant\n%s", strings.Join(got, " "), want)
	}
	if r := cartao.Rows[0]; r.AmountCents != -7490 || r.Date != "2026-10-03" {
		t.Errorf("compra no cartão = %+v", r)
	}
}

func TestFetchErrors(t *testing.T) {
	srv := fakePluggy(t)
	defer srv.Close()
	ctx := context.Background()
	if _, err := Fetch(ctx, srv.Client(), srv.URL, Credentials{ClientID: "id", ClientSecret: "errado", ItemIDs: []string{"item-1"}}, "2026-09-01"); err == nil || !strings.Contains(err.Error(), "Client ID ou o Client Secret") {
		t.Errorf("credencial errada = %v", err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL, Credentials{ClientID: "id", ClientSecret: "segredo"}, "2026-09-01"); err == nil {
		t.Error("sem Item ID deveria dar erro")
	}
	// A mensagem de erro nunca leva o segredo.
	_, err := Fetch(ctx, srv.Client(), "http://127.0.0.1:1", Credentials{ClientID: "id", ClientSecret: "segredo-xyz", ItemIDs: []string{"item-1"}}, "2026-09-01")
	if err == nil || strings.Contains(err.Error(), "segredo-xyz") {
		t.Errorf("erro de conexão = %v", err)
	}
}
