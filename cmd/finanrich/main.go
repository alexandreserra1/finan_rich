// Comando finanrich: sobe o servidor web do controle financeiro.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/config"
	"github.com/alexandreserra1/finan_rich/internal/store"
	"github.com/alexandreserra1/finan_rich/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	var adv *advisor.Advisor
	if cfg.AnthropicKey != "" {
		adv = advisor.New(cfg.AnthropicKey, cfg.ClaudeModel)
	} else {
		log.Print("ANTHROPIC_API_KEY não definida: o consultor IA ficará desativado")
	}

	srv, err := web.New(st, adv)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Addr: cfg.Addr, Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	log.Printf("finan_rich rodando em http://%s", cfg.Addr)
	if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
