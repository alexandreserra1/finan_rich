package web

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// tailscaleURL devolve o endereço HTTPS deste computador na rede Tailscale do
// usuário (ex.: https://meu-pc.tail1234.ts.net), se o Tailscale estiver
// instalado e conectado. Com HTTPS, o celular consegue instalar o app e abri-lo
// sem conexão; e acessa de qualquer lugar, só pelos aparelhos do usuário.
func tailscaleURL(ctx context.Context) string {
	bin := tailscaleBinary()
	if bin == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "status", "--json").Output()
	if err != nil {
		return ""
	}
	var st struct {
		BackendState string
		Self         struct{ DNSName string }
	}
	if json.Unmarshal(out, &st) != nil || st.BackendState != "Running" || st.Self.DNSName == "" {
		return ""
	}
	return "https://" + strings.TrimSuffix(st.Self.DNSName, ".")
}

func tailscaleBinary() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	if runtime.GOOS == "darwin" {
		const app = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
		if _, err := os.Stat(app); err == nil {
			return app
		}
	}
	return ""
}
