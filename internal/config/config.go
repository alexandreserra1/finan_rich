// Package config lê a configuração inicial do app a partir de flags e variáveis
// de ambiente. As configurações da IA ficam no banco e são editadas na tela de
// Configurações; as variáveis FINAN_AI_* só definem os valores iniciais.
package config

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

type Config struct {
	Addr        string     // endereço HTTP, ex: 127.0.0.1:8080
	DataDir     string     // pasta onde fica o banco finan.db
	OpenBrowser bool       // abrir o navegador ao iniciar
	AIPreset    string     // preset inicial da IA (ver llm.Presets)
	AI          llm.Config // valores iniciais da IA
}

// DBPath é o caminho do arquivo do banco de dados.
func (c Config) DBPath() string { return filepath.Join(c.DataDir, "finan.db") }

// Load lê flags (com prioridade) e variáveis de ambiente.
func Load(args []string) (Config, error) {
	preset := llm.PresetByID(env("FINAN_AI_PRESET", "ollama"))
	cfg := Config{AIPreset: preset.ID}

	fs := flag.NewFlagSet("finanrich", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", env("FINAN_ADDR", "127.0.0.1:8080"), "endereço HTTP")
	fs.StringVar(&cfg.DataDir, "data", env("FINAN_DATA_DIR", defaultDataDir()), "pasta dos dados")
	noBrowser := fs.Bool("no-browser", os.Getenv("FINAN_NO_BROWSER") != "", "não abrir o navegador ao iniciar")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.OpenBrowser = !*noBrowser
	cfg.AI = llm.Config{
		Kind:    preset.Kind,
		BaseURL: env("FINAN_AI_BASE_URL", preset.BaseURL),
		Model:   env("FINAN_AI_MODEL", preset.Model),
		APIKey:  os.Getenv("FINAN_AI_API_KEY"),
	}
	return cfg, nil
}

// defaultDataDir segue o padrão de cada sistema:
// macOS ~/Library/Application Support/finan_rich, Windows %AppData%\finan_rich,
// Linux ~/.config/finan_rich.
func defaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "finan_rich_dados"
	}
	return filepath.Join(dir, "finan_rich")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
