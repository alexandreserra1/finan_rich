// Package config lê a configuração da aplicação a partir de variáveis de ambiente.
package config

import "os"

type Config struct {
	Addr         string // endereço HTTP, ex: 127.0.0.1:8080
	DatabaseURL  string // ex: postgres://finan:finan@localhost:5432/finan?sslmode=disable
	AnthropicKey string // ANTHROPIC_API_KEY (opcional: sem ela o chat fica desativado)
	ClaudeModel  string
}

func Load() Config {
	return Config{
		Addr:         env("ADDR", "127.0.0.1:8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://finan:finan@localhost:5432/finan?sslmode=disable"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:  env("CLAUDE_MODEL", "claude-opus-5-5"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
