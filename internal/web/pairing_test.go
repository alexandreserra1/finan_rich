package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// api faz uma chamada JSON como o app do celular (outra origem, com token).
func (a *accessApp) api(method, path, from, token, body string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = from
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "capacitor://localhost")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func (a *accessApp) pair(code string) (string, int) {
	a.t.Helper()
	rec := a.api("POST", "/api/parear", phone, "", `{"code":"`+code+`","name":"Android"}`)
	var res pairResponse
	json.Unmarshal(rec.Body.Bytes(), &res)
	return res.Token, rec.Code
}

func (a *accessApp) newCode() string {
	a.t.Helper()
	if rec := a.do("POST", "/configuracoes/parear", local, url.Values{}); rec.Code != http.StatusSeeOther {
		a.t.Fatalf("gerar código = %d", rec.Code)
	}
	code, _, ok := a.srv.pairing.active(time.Now())
	if !ok {
		a.t.Fatal("nenhum código ativo")
	}
	return code
}

const emptySync = `{"cursor":0,"changes":[]}`

func TestPairingFlow(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")

	if _, status := a.pair("AAAA-AAAA"); status != http.StatusForbidden {
		t.Errorf("sem código ativo = %d", status)
	}
	code := a.newCode()
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, formatCode(code)) || !strings.Contains(body, "data:image/png") {
		t.Error("Ajustes deveria mostrar o código e o QR")
	}
	page := a.do("GET", "/parear?codigo="+code, phone, nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "finanrich://parear?codigo="+code+"&amp;url=http%3A%2F%2Fexample.com") {
		t.Errorf("página /parear: %d\n%s", page.Code, page.Body.String())
	}

	// Digitado com minúsculas e hífen também vale; depois de usado, não vale mais.
	token, status := a.pair(strings.ToLower(formatCode(code)))
	if status != http.StatusOK || token == "" {
		t.Fatalf("parear = %d", status)
	}
	if _, status := a.pair(code); status != http.StatusForbidden {
		t.Errorf("código reaproveitado = %d", status)
	}

	rec := a.api("POST", "/api/sync", phone, token, emptySync)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("sync com token = %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.api("OPTIONS", "/api/sync", phone, "", ""); rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("preflight = %d", rec.Code)
	}
	if rec := a.api("POST", "/api/sync", phone, "token-falso", emptySync); rec.Code != http.StatusUnauthorized {
		t.Errorf("token falso = %d", rec.Code)
	}
	// Com token, o acesso local não vale: uma página de outro site aberta neste
	// computador não pode usar o atalho de "sem senha no computador".
	if rec := a.api("POST", "/api/sync", local, "token-falso", emptySync); rec.Code != http.StatusUnauthorized {
		t.Errorf("token falso vindo do computador = %d", rec.Code)
	}
	// Sem token, outra origem continua barrada (CSRF).
	if rec := a.api("POST", "/api/sync", local, "", emptySync); rec.Code != http.StatusForbidden {
		t.Errorf("outra origem sem token = %d", rec.Code)
	}

	devices, _ := a.st.Devices(t.Context())
	if len(devices) != 1 || devices[0].Name != "Android" || devices[0].LastSeen.IsZero() {
		t.Fatalf("aparelhos = %+v", devices)
	}
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "Android") {
		t.Error("Ajustes deveria listar o aparelho")
	}
	a.do("POST", "/configuracoes/aparelhos/remover", local, url.Values{"id": {"1"}})
	if rec := a.api("POST", "/api/sync", phone, token, emptySync); rec.Code != http.StatusUnauthorized {
		t.Errorf("aparelho removido = %d", rec.Code)
	}
}

func TestPairingRevokedOnPasswordChange(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	token, _ := a.pair(a.newCode())
	a.do("POST", "/configuracoes/acesso", local, url.Values{"acao": {"senha"}, "senha": {"senha-forte-2"}, "confirmar": {"senha-forte-2"}})
	if rec := a.api("POST", "/api/sync", phone, token, emptySync); rec.Code != http.StatusUnauthorized {
		t.Errorf("token depois de trocar a senha = %d", rec.Code)
	}
}

func TestPairingRateLimit(t *testing.T) {
	a := newAccessApp(t, "")
	a.enableLAN("senha-forte-1")
	code := a.newCode()
	for range 5 {
		a.pair("ERRADO22")
	}
	if _, status := a.pair(code); status != http.StatusTooManyRequests {
		t.Errorf("depois de 5 erros = %d", status)
	}
}

func TestPairingNeedsAddress(t *testing.T) {
	a := newAccessApp(t, "")
	if body := a.do("GET", "/configuracoes", local, nil).Body.String(); !strings.Contains(body, "Primeiro ligue o") {
		t.Error("sem acesso pela rede, Ajustes deveria pedir para ligar")
	}
}
