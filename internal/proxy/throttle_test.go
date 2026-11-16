package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucketRefill(test *testing.T) {
	now := time.Unix(100, 0)
	throttle := newTokenBucket(2, 6*time.Second, 3, func() time.Time { return now })
	for range 2 {
		if wait := throttle.allow("ip"); wait != 0 {
			test.Fatalf("rajada recusada: %v", wait)
		}
	}
	if wait := throttle.allow("ip"); wait != 6*time.Second {
		test.Fatalf("espera = %v", wait)
	}
	now = now.Add(5500 * time.Millisecond)
	if wait := throttle.allow("ip"); wait != 500*time.Millisecond {
		test.Fatalf("espera fracionada = %v", wait)
	}
	now = now.Add(-time.Hour)
	if wait := throttle.allow("ip"); wait != 500*time.Millisecond {
		test.Fatalf("relógio recuou: %v", wait)
	}
	now = time.Unix(106, 0)
	if wait := throttle.allow("ip"); wait != 0 {
		test.Fatalf("reposição antes do consumo: %v", wait)
	}
	now = now.Add(24 * time.Hour)
	for range 2 {
		if wait := throttle.allow("ip"); wait != 0 {
			test.Fatal(wait)
		}
	}
	if wait := throttle.allow("ip"); wait != 6*time.Second {
		test.Fatalf("saldo passou do teto: %v", wait)
	}
}

func TestTokenBucketConcurrentAndCapacity(test *testing.T) {
	now := time.Unix(100, 0)
	throttle := newTokenBucket(5, time.Second, 2, func() time.Time { return now })
	var admitted atomic.Int64
	var group sync.WaitGroup
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			if throttle.allow("primeiro") == 0 {
				admitted.Add(1)
			}
		}()
	}
	group.Wait()
	if admitted.Load() != 5 {
		test.Fatalf("aceitos = %d", admitted.Load())
	}
	if throttle.allow("segundo") != 0 {
		test.Fatal("segunda chave recusada")
	}
	if wait := throttle.allow("terceiro"); wait != time.Second {
		test.Fatalf("capacidade: espera = %v", wait)
	}
	if wait := throttle.allow("primeiro"); wait != time.Second {
		test.Fatalf("capacidade soltou bloqueio: %v", wait)
	}
	now = now.Add(time.Second)
	if throttle.allow("terceiro") != 0 {
		test.Fatal("balde cheio não foi removido")
	}
	if len(throttle.buckets) != 2 || len(throttle.expiry) != 2 {
		test.Fatal("memória excedeu teto")
	}
	if throttle.allow("primeiro") != 0 || throttle.allow("primeiro") == 0 {
		test.Fatal("evicção reiniciou balde ativo")
	}
}

type countedBody struct{ reads int }

func (throttle *countedBody) Read([]byte) (int, error) { throttle.reads++; return 0, io.EOF }
func (*countedBody) Close() error                      { return nil }

func TestThrottleBeforeBodyAndNetwork(test *testing.T) {
	for _, paths := range [][]string{{"/chat/completions", "/v1/chat/completions"}, {"/oauth/token", "/oauth/refresh", "/oauth/logout"}} {
		test.Run(paths[0], func(test *testing.T) {
			var calls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(recorder http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				io.WriteString(recorder, `{}`)
			}))
			defer upstream.Close()
			now := time.Unix(100, 0)
			proxyHandler := newProxy(Config{Upstream: upstream.URL, Models: []string{"m"}, Limits: DefaultLimits, GoogleClientID: "id", GoogleClientSecret: "s", GoogleTokenURL: upstream.URL, GoogleRevokeURL: upstream.URL}, func() time.Time { return now })
			burst, retry := 5, "1"
			if strings.HasPrefix(paths[0], "/oauth/") {
				burst, retry = 10, "6"
			}
			for index := range burst + 1 {
				body := &countedBody{}
				request := httptest.NewRequest("POST", paths[index%len(paths)], body)
				request.Header.Set("Authorization", fmt.Sprintf("Bearer inventado-%d", index))
				request.AddCookie(&http.Cookie{Name: rtCookie, Value: "revogar"})
				recorder := httptest.NewRecorder()
				proxyHandler.ServeHTTP(recorder, request)
				if index == burst {
					if body.reads != 0 {
						test.Fatal("corpo recusado foi lido")
					}
					assertLimited(test, recorder, retry)
				} else if recorder.Code == 429 {
					test.Fatalf("rajada legítima recusada no pedido %d", index+1)
				}
			}
			before := calls.Load()
			for _, path := range paths {
				request := httptest.NewRequest("POST", path, strings.NewReader(appBody("m", 10)))
				request.AddCookie(&http.Cookie{Name: rtCookie, Value: "revogar"})
				recorder := httptest.NewRecorder()
				proxyHandler.ServeHTTP(recorder, request)
				assertLimited(test, recorder, retry)
			}
			if calls.Load() != before {
				test.Fatal("negação chamou upstream")
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", paths[0], strings.NewReader(`{}`))
			request.RemoteAddr = "203.0.113.2:1234"
			proxyHandler.ServeHTTP(recorder, request)
			if recorder.Code == 429 {
				test.Fatal("outro IP foi bloqueado")
			}
			for _, path := range []string{"/healthz", "/oauth/config"} {
				recorder := httptest.NewRecorder()
				proxyHandler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
				if recorder.Code != 200 {
					test.Fatalf("GET %s bloqueado: %d", path, recorder.Code)
				}
			}
		})
	}
}

func assertLimited(test *testing.T, recorder *httptest.ResponseRecorder, retry string) {
	test.Helper()
	if recorder.Code != 429 || recorder.Header().Get("Retry-After") != retry || recorder.Header().Get("Cache-Control") != "no-store" {
		test.Fatalf("limite = %d, cabeçalhos = %v", recorder.Code, recorder.Header())
	}
	if recorder.Header().Get("Access-Control-Expose-Headers") != "Retry-After" {
		test.Fatal("Retry-After não exposto via CORS")
	}
}

func TestQuotaRetryAfter(test *testing.T) {
	for _, dimension := range []string{"aparelho/hora", "aparelho/dia", "IP/hora", "IP/dia", "hora e dia"} {
		test.Run(dimension, func(test *testing.T) {
			now := time.Date(2026, 10, 22, 10, 59, 59, 500000000, time.UTC)
			limits := Limits{100, 100, 100, 100}
			want := 500 * time.Millisecond
			switch dimension {
			case "aparelho/hora":
				limits.DeviceHour = 1
			case "aparelho/dia":
				limits.DeviceDay = 1
				want = 13*time.Hour + 500*time.Millisecond
			case "IP/hora":
				limits.IPHour = 1
			case "IP/dia":
				limits.IPDay = 1
				want = 13*time.Hour + 500*time.Millisecond
			case "hora e dia":
				limits.IPHour, limits.DeviceDay = 1, 1
				want = 13*time.Hour + 500*time.Millisecond
			}
			quotaLimiter := newLimiter(func() time.Time { return now })
			if msg, wait := quotaLimiter.allowWithRetry("d", "203.0.113.1", limits); msg != "" || wait != 0 {
				test.Fatalf("primeiro pedido = %q, %v", msg, wait)
			}
			msg, wait := quotaLimiter.allowWithRetry("d", "203.0.113.1", limits)
			if msg == "" || wait != want {
				test.Fatalf("negação = %q, %v; quer %v", msg, wait, want)
			}
			recorder := httptest.NewRecorder()
			rateLimited(recorder, msg, wait)
			retry := "1"
			if want > time.Second {
				retry = "46801"
			}
			if recorder.Header().Get("Retry-After") != retry || recorder.Header().Get("Cache-Control") != "no-store" {
				test.Fatal(recorder.Header())
			}
			now = now.Add(wait)
			if msg, _ := quotaLimiter.allowWithRetry("d", "203.0.113.1", limits); msg != "" {
				test.Fatalf("janela nova bloqueada: %s", msg)
			}
		})
	}
}

func TestThrottleLegitimateFlow(test *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(recorder http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/models") {
			http.NotFound(recorder, request)
			return
		}
		calls.Add(1)
		io.WriteString(recorder, `{}`)
	}))
	defer upstream.Close()
	now := time.Unix(100, 0)
	proxyHandler := newProxy(Config{Upstream: upstream.URL, Models: []string{"m"}, Limits: DefaultLimits, GoogleClientID: "id", GoogleClientSecret: "s", GoogleTokenURL: upstream.URL, GoogleRevokeURL: upstream.URL}, func() time.Time { return now })
	for range 5 {
		if recorder := ask(proxyHandler, "d", appBody("m", 10)); recorder.Code != 200 {
			test.Fatalf("rodada legítima do consultor = %d", recorder.Code)
		}
	}
	for _, path := range []string{"/oauth/token", "/oauth/refresh", "/oauth/logout"} {
		request := httptest.NewRequest("POST", path, strings.NewReader(`{"code":"c","verifier":"v","redirectUri":"https://site/oauth","refreshToken":"r"}`))
		request.Header.Set("X-Forwarded-For", "200.1.2.3")
		request.AddCookie(&http.Cookie{Name: rtCookie, Value: "r"})
		recorder := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			test.Fatalf("fluxo legítimo %s = %d", path, recorder.Code)
		}
	}
	assertLimited(test, ask(proxyHandler, "outro-bearer", appBody("m", 10)), "1")
	now = now.Add(999 * time.Millisecond)
	assertLimited(test, ask(proxyHandler, "d", appBody("m", 10)), "1")
	now = now.Add(time.Millisecond)
	if recorder := ask(proxyHandler, "d", appBody("m", 10)); recorder.Code != 200 {
		test.Fatalf("chat não retomou: %d", recorder.Code)
	}
	if calls.Load() != 9 {
		test.Fatalf("chamadas = %d, quer 9", calls.Load())
	}
}

func TestChatQuotaAvoidsUpstream(test *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(recorder http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		io.WriteString(recorder, `{}`)
	}))
	defer upstream.Close()
	now := time.Date(2026, 10, 22, 10, 0, 0, 500000000, time.UTC)
	proxyHandler := newProxy(Config{Upstream: upstream.URL, Models: []string{"m"}, Limits: Limits{1, 10, 10, 10}}, func() time.Time { return now })
	if recorder := ask(proxyHandler, "d", appBody("m", 10)); recorder.Code != 200 {
		test.Fatal(recorder.Code)
	}
	before := calls.Load()
	assertLimited(test, ask(proxyHandler, "d", appBody("m", 10)), "3600")
	if calls.Load() != before {
		test.Fatal("cota negada consultou provedor/modelos")
	}
}

func TestTokenBucketIPv6AndHeapUpdates(test *testing.T) {
	now := time.Unix(100, 0)
	throttle := newTokenBucket(2, time.Second, 2, func() time.Time { return now })
	if throttle.allow("2001:db8:1:2::1") != 0 || throttle.allow("2001:db8:1:2::2") != 0 || throttle.allow("2001:db8:1:2::3") != time.Second {
		test.Fatal("IPv6 burlou balde do /64")
	}
	throttle.allow("segundo")
	throttle.allow("segundo")
	if throttle.allow("terceiro") != 2*time.Second {
		test.Fatal("espera de capacidade incorreta")
	}
	now = now.Add(time.Second)
	throttle.allow("2001:db8:1:2::1")
	now = now.Add(time.Second)
	if throttle.allow("terceiro") != 0 {
		test.Fatal("heap não moveu entrada consumida")
	}
	if _, ok := throttle.buckets["segundo"]; ok {
		test.Fatal("balde cheio não removido")
	}
	if throttle.allow("2001:db8:1:2::4") != 0 || throttle.allow("2001:db8:1:2::5") == 0 {
		test.Fatal("balde IPv6 ativo reiniciado")
	}
}

func TestTokenBucketMaxKeys(test *testing.T) {
	now := time.Unix(100, 0)
	throttle := newTokenBucket(5, time.Second, maxBurstKeys, func() time.Time { return now })
	for index := range maxBurstKeys {
		if throttle.allow(fmt.Sprintf("ip-%d", index)) != 0 {
			test.Fatalf("chave %d recusada antes do teto", index)
		}
	}
	for range 100 {
		if wait := throttle.allow("novo"); wait != time.Second {
			test.Fatalf("espera de capacidade = %v", wait)
		}
	}
	if len(throttle.buckets) != maxBurstKeys || len(throttle.expiry) != maxBurstKeys {
		test.Fatal("negações aumentaram a memória")
	}
	now = now.Add(time.Second)
	if throttle.allow("novo") != 0 || len(throttle.buckets) != maxBurstKeys || len(throttle.expiry) != maxBurstKeys {
		test.Fatal("reciclagem não manteve o teto")
	}
}

func TestOAuthQuotaBeforeBodyAndNetwork(test *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		io.WriteString(writer, `{}`)
	}))
	defer upstream.Close()
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	proxyHandler := newProxy(Config{GoogleClientID: "id", GoogleClientSecret: "s", GoogleTokenURL: upstream.URL, GoogleRevokeURL: upstream.URL}, func() time.Time { return now })
	for index := range oauthLimits.DeviceHour {
		path := []string{"/oauth/token", "/oauth/refresh", "/oauth/logout"}[index%3]
		request := httptest.NewRequest("POST", path, strings.NewReader(`{"code":"c","verifier":"v","redirectUri":"https://site/oauth","refreshToken":"r"}`))
		request.AddCookie(&http.Cookie{Name: rtCookie, Value: "r"})
		recorder := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			test.Fatalf("pedido %d recusado antes da cota: %d", index+1, recorder.Code)
		}
		now = now.Add(6 * time.Second)
	}
	for _, path := range []string{"/oauth/token", "/oauth/refresh", "/oauth/logout"} {
		body := &countedBody{}
		request := httptest.NewRequest("POST", path, body)
		request.AddCookie(&http.Cookie{Name: rtCookie, Value: "r"})
		recorder := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recorder, request)
		assertLimited(test, recorder, "3420")
		if body.reads != 0 {
			test.Fatal("negação leu corpo")
		}
		if path == "/oauth/logout" {
			assertExpiredRT(test, recorder)
		} else if recorder.Header().Get("Set-Cookie") != "" {
			test.Fatal("troca ou renovação recusada alterou cookie")
		}
	}
	if calls.Load() != int64(oauthLimits.DeviceHour) {
		test.Fatalf("negação chamou Google: %d chamadas", calls.Load())
	}
}

func TestLimitedLogoutClearsCookie(test *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		io.WriteString(writer, `{}`)
	}))
	defer upstream.Close()
	proxyHandler := newProxy(Config{GoogleClientID: "id", GoogleClientSecret: "s", GoogleTokenURL: upstream.URL, GoogleRevokeURL: upstream.URL}, func() time.Time { return time.Unix(100, 0) })
	for range 10 {
		proxyHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/oauth/token", strings.NewReader(`{}`)))
	}
	body := &countedBody{}
	request := httptest.NewRequest("POST", "/oauth/logout", body)
	request.AddCookie(&http.Cookie{Name: rtCookie, Value: "token-falso"})
	recorder := httptest.NewRecorder()
	proxyHandler.ServeHTTP(recorder, request)
	assertLimited(test, recorder, "6")
	assertExpiredRT(test, recorder)
	if !strings.Contains(recorder.Body.String(), "não foi revogado") {
		test.Fatal("logout limitado prometeu revogação no Google")
	}
	if calls.Load() != 0 || body.reads != 0 {
		test.Fatal("logout limitado leu corpo ou chamou provedor")
	}
}

func assertExpiredRT(test *testing.T, recorder *httptest.ResponseRecorder) {
	test.Helper()
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != rtCookie || cookies[0].Value != "" || cookies[0].MaxAge >= 0 ||
		!cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		test.Fatal("logout recusado manteve a sessão no navegador")
	}
}
