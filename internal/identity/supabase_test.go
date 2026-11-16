package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testID = "12345678-1234-4234-8234-123456789abc"
const testKey = "sb_publishable_chave-publica-de-teste"
const testPassword = " senha de teste "
const verifiedUser = `{"id":"` + testID + `","email":"pessoa@example.com","email_confirmed_at":"2026-01-01T12:00:00Z"}`

func testSupabase(t *testing.T, handler http.HandlerFunc) *Supabase {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	adapter, err := NewSupabase(server.URL, testKey, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestAuthenticateContract(t *testing.T) {
	var requests []string
	adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if request.Header.Get("apikey") != testKey || request.Header.Get("Cookie") != "" {
			t.Error("cabeçalhos inesperados")
		}
		switch request.URL.Path {
		case "/auth/v1/token":
			var credentials map[string]string
			if request.Method != "POST" || request.URL.RawQuery != "grant_type=password" || request.Header.Get("Authorization") != "" || request.Header.Get("Content-Type") != "application/json" {
				t.Error("contrato do login incorreto")
			}
			if json.NewDecoder(request.Body).Decode(&credentials) != nil || credentials["password"] != testPassword || credentials["email"] != "pessoa@example.com" || len(credentials) != 2 {
				t.Error("credenciais alteradas ou campos extras")
			}
			w.Write([]byte(`{"access_token":"token-opaco-de-teste","refresh_token":"nao-retornar","token_type":"bearer","user":{"id":"nao-confiar"}}`))
		case "/auth/v1/user":
			if request.Method != "GET" || request.Header.Get("Authorization") != "Bearer token-opaco-de-teste" {
				t.Error("verificação do usuário incorreta")
			}
			w.Write([]byte(verifiedUser))
		default:
			t.Error("rota inesperada")
		}
	})
	for range 2 {
		person, err := adapter.Authenticate(context.Background(), " pessoa@example.com ", testPassword)
		if err != nil || person != (Identity{ID: testID, Email: "pessoa@example.com"}) {
			t.Fatalf("identidade verificada incorreta: %v", err)
		}
	}
	if len(requests) != 4 {
		t.Fatalf("esperava nova verificação em cada login: %d", len(requests))
	}
}

func TestSignupContract(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{"novo", 200, `{"id":"` + testID + `","email_confirmed_at":null}`},
		{"duplicado ofuscado", 200, `{"id":"` + testID + `","identities":[]}`},
		{"duplicado 400", 400, `{"error_code":"user_already_exists","msg":"dado privado"}`},
		{"duplicado 422", 422, `{"error_code":"user_already_exists"}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
				var credentials map[string]string
				if request.Method != "POST" || request.URL.RequestURI() != "/auth/v1/signup" || request.Header.Get("apikey") != testKey || request.Header.Get("Authorization") != "" {
					t.Error("contrato do cadastro incorreto")
				}
				if json.NewDecoder(request.Body).Decode(&credentials) != nil || credentials["password"] != testPassword || credentials["email"] != "pessoa@example.com" || len(credentials) != 2 {
					t.Error("cadastro alterou credenciais")
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(scenario.status)
				w.Write([]byte(scenario.body))
			})
			if err := adapter.Signup(context.Background(), " pessoa@example.com ", testPassword); err != nil {
				t.Fatalf("cadastro deveria ter resultado neutro: %v", err)
			}
		})
	}
}

func TestProviderFailures(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		path   string
		status int
		body   string
		want   error
	}{
		{"senha incorreta", "token", 400, `{"error_code":"invalid_credentials"}`, ErrCredentials},
		{"email pendente", "token", 422, `{"error_code":"email_not_confirmed"}`, ErrCredentials},
		{"usuario recusado", "user", 401, `{"error_code":"bad_jwt"}`, ErrCredentials},
		{"usuario sem confirmacao", "user", 200, `{"id":"` + testID + `","email":"pessoa@example.com"}`, ErrCredentials},
		{"confirmacao invalida", "user", 200, `{"id":"` + testID + `","email":"pessoa@example.com","email_confirmed_at":"ontem"}`, ErrUnavailable},
		{"id ausente", "user", 200, `{"email":"pessoa@example.com"}`, ErrUnavailable},
		{"id invalido", "user", 200, `{"id":"outro"}`, ErrUnavailable},
		{"email invalido", "user", 200, `{"id":"` + testID + `","email":"invalido"}`, ErrUnavailable},
		{"json malformado", "token", 200, `{`, ErrUnavailable},
		{"json concatenado", "user", 200, verifiedUser + `{}`, ErrUnavailable},
		{"resposta nula", "token", 200, `null`, ErrUnavailable},
		{"token ausente", "token", 200, `{}`, ErrUnavailable},
		{"tipo token invalido", "token", 200, `{"access_token":"segredo","token_type":"outro"}`, ErrUnavailable},
		{"corpo grande", "user", 200, `{"privado":"` + strings.Repeat("x", 65536) + `"}`, ErrUnavailable},
		{"limite", "token", 429, `{"msg":"segredo"}`, ErrUnavailable},
		{"servidor", "user", 503, `{"msg":"segredo"}`, ErrUnavailable},
		{"status estranho", "token", 201, `{"access_token":"segredo"}`, ErrUnavailable},
		{"sem corpo", "user", 204, "", ErrUnavailable},
		{"cadastro malformado", "signup", 200, `{}`, ErrUnavailable},
		{"cadastro sem confirmacao exigida", "signup", 200, `{"access_token":"segredo","refresh_token":"segredo"}`, ErrUnavailable},
		{"cadastro ja confirmado", "signup", 200, verifiedUser, ErrUnavailable},
		{"duplicado status estranho", "signup", 500, `{"error_code":"user_already_exists"}`, ErrUnavailable},
		{"cadastro recusado", "signup", 422, `{"error_code":"weak_password","msg":"segredo"}`, ErrUnavailable},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if request.URL.Path == "/auth/v1/"+scenario.path {
					w.WriteHeader(scenario.status)
					w.Write([]byte(scenario.body))
					return
				}
				w.Write([]byte(`{"access_token":"segredo","token_type":"bearer"}`))
			})
			var err error
			if scenario.path == "signup" {
				err = adapter.Signup(context.Background(), "pessoa@example.com", testPassword)
			} else {
				var person Identity
				person, err = adapter.Authenticate(context.Background(), "pessoa@example.com", testPassword)
				if person != (Identity{}) {
					t.Error("falha retornou identidade")
				}
			}
			if !errors.Is(err, scenario.want) {
				t.Fatalf("esperava erro genérico %v, recebeu %v", scenario.want, err)
			}
		})
	}
}

func TestInvalidInput(t *testing.T) {
	adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) { t.Error("entrada inválida chegou ao provedor") })
	for _, input := range []struct{ email, password string }{
		{"", testPassword}, {"Nome <pessoa@example.com>", testPassword}, {"pessoa@example.com\r\n", testPassword},
		{"sem-arroba", testPassword}, {"pessoa@example.com", ""}, {"pessoa@example.com", strings.Repeat("a", 73)},
		{"pessoa@example.com", strings.Repeat("á", 37)}, {"pessoa@example.com", "invalida\x00senha"},
	} {
		if _, err := adapter.Authenticate(context.Background(), input.email, input.password); !errors.Is(err, ErrInput) {
			t.Error("login aceitou entrada inválida")
		}
		if err := adapter.Signup(context.Background(), input.email, input.password); !errors.Is(err, ErrInput) {
			t.Error("cadastro aceitou entrada inválida")
		}
	}
	for _, password := range []string{"curta", strings.Repeat("a", 14), strings.Repeat("á", 14)} {
		if err := adapter.Signup(context.Background(), "pessoa@example.com", password); !errors.Is(err, ErrInput) {
			t.Error("cadastro aceitou senha com menos de 15 caracteres")
		}
	}
}

func TestHTTPSAndConfiguration(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "https://usuario:senha@example.com", "https://example.com?token=segredo", "https://example.com/#frag", "https://example.com/auth/v1", "https:///"} {
		if _, err := NewSupabase(endpoint, testKey, nil); !errors.Is(err, ErrConfiguration) {
			t.Error("aceitou endereço inválido")
		}
	}
	for _, key := range []string{"", "sb_secret_nao-usar", "sb_publishable_", "sb_publishable_teste\r\n"} {
		if _, err := NewSupabase("https://example.com", key, nil); !errors.Is(err, ErrConfiguration) {
			t.Error("aceitou chave inválida")
		}
	}
}

func TestRedirectIsNeverFollowed(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, phase := range []string{"signup", "token", "user"} {
			var leaked atomic.Int32
			adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/roubar" {
					leaked.Add(1)
				}
				if phase == "user" && request.URL.Path == "/auth/v1/token" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"access_token":"segredo","token_type":"bearer"}`))
					return
				}
				w.Header().Set("Location", "/roubar")
				w.WriteHeader(status)
			})
			var err error
			if phase == "signup" {
				err = adapter.Signup(context.Background(), "pessoa@example.com", testPassword)
			} else {
				_, err = adapter.Authenticate(context.Background(), "pessoa@example.com", testPassword)
			}
			if !errors.Is(err, ErrUnavailable) || leaked.Load() != 0 {
				t.Errorf("redirecionamento %d em %s não foi bloqueado", status, phase)
			}
		}
	}
}

func TestPasswordBoundaries(t *testing.T) {
	for _, password := range []string{strings.Repeat("a", 15), strings.Repeat("á", 15), strings.Repeat("a", 72), strings.Repeat("á", 36), "x"} {
		adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if request.Method == "POST" {
				var input credentials
				if json.NewDecoder(request.Body).Decode(&input) != nil || input.Password != password {
					t.Error("senha não foi preservada")
				}
			}
			switch request.URL.Path {
			case "/auth/v1/signup":
				w.Write([]byte(`{"id":"` + testID + `"}`))
			case "/auth/v1/token":
				w.Write([]byte(`{"access_token":"segredo","token_type":"bearer"}`))
			case "/auth/v1/user":
				w.Write([]byte(verifiedUser))
			}
		})
		if password != "x" {
			if err := adapter.Signup(context.Background(), "pessoa@example.com", password); err != nil {
				t.Fatalf("cadastro rejeitou senha dentro dos limites: %v", err)
			}
		}
		if _, err := adapter.Authenticate(context.Background(), "pessoa@example.com", password); err != nil {
			t.Fatalf("login rejeitou senha existente: %v", err)
		}
	}
}

func TestClientPolicy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"` + testID + `"}`))
	}))
	defer server.Close()
	client := server.Client()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	client.Timeout = time.Hour
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	adapter, err := NewSupabase(server.URL, testKey, client)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.client.Jar != nil || adapter.client.Timeout != requestTimeout || adapter.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("cliente não aplicou política de segurança")
	}
	if client.Jar != jar || client.Timeout != time.Hour || client.CheckRedirect(nil, nil) != nil {
		t.Error("construtor alterou cliente do chamador")
	}
	untrusted, err := NewSupabase(server.URL, testKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := untrusted.Signup(context.Background(), "pessoa@example.com", testPassword); !errors.Is(err, ErrUnavailable) {
		t.Error("aceitou certificado não confiável")
	}
}

func TestTimeoutAndContentType(t *testing.T) {
	adapter := testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Content-Type") == "application/json" {
			select {
			case <-request.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := adapter.Authenticate(ctx, "pessoa@example.com", testPassword); !errors.Is(err, ErrUnavailable) {
		t.Fatal("timeout deveria retornar erro genérico")
	}
	adapter = testSupabase(t, func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(verifiedUser))
	})
	if err := adapter.Signup(context.Background(), "pessoa@example.com", testPassword); !errors.Is(err, ErrUnavailable) {
		t.Error("aceitou resposta sem tipo JSON")
	}
}
