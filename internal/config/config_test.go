package config

import (
	"path/filepath"
	"testing"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FINAN_AI_PRESET", "")
	t.Setenv("FINAN_AI_MODEL", "")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:8080" || !cfg.OpenBrowser || cfg.AIPreset != "ollama" {
		t.Errorf("padrões = %+v", cfg)
	}
	if cfg.AI.Kind != llm.KindOllama || cfg.AI.Model == "" || cfg.AI.BaseURL != llm.DefaultOllamaURL {
		t.Errorf("IA padrão = %+v", cfg.AI)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FINAN_AI_PRESET", "groq")
	t.Setenv("FINAN_AI_API_KEY", "k")
	dir := t.TempDir()
	cfg, err := Load([]string{"-addr", ":9000", "-data", dir, "-no-browser"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9000" || cfg.OpenBrowser || cfg.DBPath() != filepath.Join(dir, "finan.db") {
		t.Errorf("flags = %+v", cfg)
	}
	if cfg.AIPreset != "groq" || cfg.AI.Kind != llm.KindOpenAI || cfg.AI.APIKey != "k" {
		t.Errorf("env = %+v", cfg.AI)
	}
}
