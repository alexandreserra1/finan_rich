package web

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/rates"
)

// ratesCache guarda as taxas do Banco Central por 6 horas. Se a busca falhar,
// tenta de novo só depois de 10 minutos; enquanto isso o consultor segue com as
// últimas que tiver (ou sem taxas). Nenhum dado da pessoa vai junto.
type ratesCache struct {
	url  string
	mu   sync.Mutex
	last rates.Rates
	next time.Time
}

func (c *ratesCache) get(ctx context.Context) rates.Rates {
	if c.url == "" {
		return rates.Rates{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.next) {
		return c.last
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second) // a pergunta não espera mais que isso
	defer cancel()
	r, err := rates.Fetch(ctx, http.DefaultClient, c.url)
	if err != nil {
		log.Printf("consultor: %v", err)
		c.next = time.Now().Add(10 * time.Minute)
		return c.last
	}
	c.last, c.next = r, time.Now().Add(6*time.Hour)
	return r
}
