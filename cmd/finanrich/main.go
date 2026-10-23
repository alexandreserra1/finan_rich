// Comando finanrich: abre o controle financeiro no navegador.
//
// É um único executável: o banco (SQLite) e a interface vêm embutidos. Basta
// executar e o navegador abre em http://127.0.0.1:8080.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/config"
	"github.com/alexandreserra1/finan_rich/internal/store"
	"github.com/alexandreserra1/finan_rich/internal/web"
)

// version é preenchida no build de release (-ldflags "-X main.version=v1.2.3").
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Print(err)
		waitBeforeExit()
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-version" || args[0] == "--version") {
		fmt.Println("finan_rich", version)
		return nil
	}
	cfg, err := config.Load(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return fmt.Errorf("endereço inválido %q: %w", cfg.Addr, err)
	}
	localURL := "http://" + net.JoinHostPort(displayHost(host), port)

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("criando a pasta de dados: %w", err)
	}
	st, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	reload := make(chan struct{}, 1)
	srv, err := web.New(st, web.Options{
		AIPreset: cfg.AIPreset, AIDefaults: cfg.AI, Port: port, Password: cfg.Password,
		OnQuit:  stop,
		Version: updateVersion(),
		OnNetworkChange: func() {
			select {
			case reload <- struct{}{}:
			default:
			}
		},
	})
	if err != nil {
		return err
	}

	// Se o usuário liberou o acesso pelo celular, escuta em todas as interfaces
	// da rede; senão, só neste computador.
	listenAddr := func() string {
		if isLoopbackHost(host) && srv.LANEnabled(ctx) {
			return net.JoinHostPort("", port)
		}
		return cfg.Addr
	}

	for first := true; ; first = false {
		addr := listenAddr()
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if first && alreadyRunning(localURL) {
				log.Printf("finan_rich já está aberto em %s", localURL)
				if cfg.OpenBrowser {
					return openApp(localURL)
				}
				return nil
			}
			return fmt.Errorf("não foi possível usar o endereço %s (outro programa está usando a porta?): %w", addr, err)
		}
		network := !isLoopbackHost(hostOf(addr))
		if first {
			printBanner(localURL, cfg.DBPath())
			if cfg.OpenBrowser {
				if err := openApp(localURL); err != nil {
					log.Printf("abra %s no navegador (%v)", localURL, err)
				}
			}
		}
		if network {
			if urls := srv.LANURLs(); len(urls) > 0 {
				fmt.Printf("  No celular (mesmo Wi-Fi): %s\n\n", strings.Join(urls, "  ou  "))
			}
			if !srv.HasPassword(ctx) {
				log.Print("ATENÇÃO: o app está acessível pela rede SEM senha. Defina FINAN_PASSWORD ou ative o acesso pelo celular nas Configurações.")
			}
		} else if !first {
			log.Print("acesso pela rede desligado: o app aceita só este computador")
		}

		// Sem WriteTimeout: o chat responde aos poucos (SSE) e pode passar de minutos.
		httpSrv := &http.Server{Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
			IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10}
		served := make(chan error, 1)
		go func() { served <- httpSrv.Serve(ln) }()

		select {
		case <-ctx.Done():
			shutdown(httpSrv)
			return nil
		case err := <-served:
			return err
		case <-reload:
			shutdown(httpSrv) // reabre no novo endereço na próxima volta
		}
	}
}

// updateVersion é a versão usada no aviso de versão nova; FINAN_UPDATE_CHECK=0
// desliga o aviso (e a consulta diária ao GitHub).
func updateVersion() string {
	if os.Getenv("FINAN_UPDATE_CHECK") == "0" {
		return ""
	}
	return version
}

func printBanner(url, dbPath string) {
	fmt.Printf("\n  finan_rich %s\n  Aberto em:  %s\n  Seus dados: %s\n  Para fechar, feche esta janela ou aperte Ctrl+C.\n\n", version, url, dbPath)
}

func shutdown(s *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

func hostOf(addr string) string {
	host, _, _ := net.SplitHostPort(addr)
	return host
}

// isLoopbackHost diz se o endereço só aceita conexões deste computador.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// displayHost troca "" e "0.0.0.0" por um endereço que o navegador local abre.
func displayHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

// alreadyRunning verifica se há um finan_rich respondendo no endereço.
func alreadyRunning(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/healthz")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// waitBeforeExit segura a janela aberta no Windows (duplo clique) para o
// usuário conseguir ler o erro.
func waitBeforeExit() {
	if isDoubleClicked() {
		fmt.Print("\nAperte Enter para fechar...")
		fmt.Scanln()
	}
}
