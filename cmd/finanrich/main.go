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
	cfg, err := config.Load(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	url := "http://" + cfg.Addr
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		// Provavelmente o app já está aberto: só abre o navegador nele.
		if alreadyRunning(url) {
			log.Printf("finan_rich já está aberto em %s", url)
			if cfg.OpenBrowser {
				return openBrowser(url)
			}
			return nil
		}
		return fmt.Errorf("não foi possível usar o endereço %s (outro programa está usando a porta?): %w", cfg.Addr, err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("criando a pasta de dados: %w", err)
	}
	st, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	srv, err := web.New(st, cfg.AIPreset, cfg.AI)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	fmt.Printf("\n  finan_rich %s\n  Aberto em:  %s\n  Seus dados: %s\n  Para fechar, feche esta janela ou aperte Ctrl+C.\n\n", version, url, cfg.DBPath())
	if cfg.OpenBrowser {
		if err := openBrowser(url); err != nil {
			log.Printf("abra %s no navegador (%v)", url, err)
		}
	}
	if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
