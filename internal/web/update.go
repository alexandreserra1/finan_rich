package web

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Aviso de versão nova. Uma vez por dia o programa pergunta ao GitHub qual é
// a última versão (só o endereço /releases/latest: nada dos seus dados sai do
// computador). Sem internet, não avisa e tenta de novo depois.
// FINAN_UPDATE_CHECK=0 desliga.

const (
	latestReleaseURL = "https://github.com/alexandreserra1/finan_rich/releases/latest"
	updateEvery      = 24 * time.Hour
)

type updateChecker struct {
	current string // versão rodando ("1.6.0"); "dev" nunca avisa
	url     string
	client  *http.Client

	mu      sync.Mutex
	latest  string
	checked time.Time
	running bool
}

func newUpdateChecker(current string) *updateChecker {
	return &updateChecker{
		current: strings.TrimPrefix(current, "v"),
		url:     latestReleaseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Não segue o redirecionamento: a versão está no endereço de destino.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// available devolve a versão nova, se houver. Nunca espera a rede: a
// consulta roda em segundo plano e o aviso aparece na página seguinte.
func (u *updateChecker) available() (string, bool) {
	if u == nil || u.current == "dev" {
		return "", false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.running && time.Since(u.checked) > updateEvery {
		u.running = true
		go u.refresh()
	}
	return u.latest, u.latest != "" && newerVersion(u.latest, u.current)
}

func (u *updateChecker) refresh() {
	latest := ""
	if res, err := u.client.Head(u.url); err == nil {
		res.Body.Close()
		_, tag, ok := strings.Cut(res.Header.Get("Location"), "/releases/tag/")
		if ok {
			latest = strings.TrimPrefix(tag, "v")
		}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.running = false
	u.checked = time.Now()
	if latest != "" {
		u.latest = latest
	}
}

// newerVersion diz se a versão a ("1.10.0") é mais nova que b ("1.9.2").
func newerVersion(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// updateCommand é o comando que atualiza neste sistema (o mesmo da instalação).
func updateCommand() string {
	if runtime.GOOS == "windows" {
		return "irm https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.ps1 | iex"
	}
	return "curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh"
}

func newUpdateCheckerIf(version string) *updateChecker {
	if version == "" {
		return nil
	}
	return newUpdateChecker(version)
}
