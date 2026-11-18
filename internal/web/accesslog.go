package web

import (
	"net"
	"net/http"
)

// logAccess grava no registro de acesso (Ajustes → Acesso pelo celular) um evento de
// segurança, com o endereço de quem fez. Nunca senha, token nem dado financeiro.
// Falhar ao gravar não muda o resultado do pedido: o registro é para conferir depois.
func (s *Server) logAccess(r *http.Request, event string) {
	s.store.LogAccess(r.Context(), event, accessFrom(r))
}

// accessFrom é de onde veio o pedido: vazio neste computador, o login do Tailscale
// (que repassa como se fosse local) ou o IP da rede.
func accessFrom(r *http.Request) string {
	if login := r.Header.Get("Tailscale-User-Login"); login != "" {
		return "Tailscale: " + login
	}
	if isLoopback(r) {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
