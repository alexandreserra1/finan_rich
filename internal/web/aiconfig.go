package web

import (
	"context"

	"github.com/alexandreserra1/finan_rich/internal/llm"
)

const aiPrefix = "ai_"

// AISettings é a configuração da IA escolhida pelo usuário.
type AISettings struct {
	Preset llm.Preset
	llm.Config
}

// aiSettings lê a configuração salva; o que não foi salvo vem dos padrões.
func (s *Server) aiSettings(ctx context.Context) (AISettings, error) {
	saved, err := s.store.Settings(ctx, aiPrefix)
	if err != nil {
		return AISettings{}, err
	}
	presetID := s.aiDefaults.Preset.ID
	cfg := s.aiDefaults.Config
	if id, ok := saved["preset"]; ok {
		presetID = id
		p := llm.PresetByID(id)
		cfg = llm.Config{Kind: p.Kind, BaseURL: saved["base_url"], Model: saved["model"], APIKey: saved["api_key"]}
	}
	return AISettings{Preset: llm.PresetByID(presetID), Config: cfg}, nil
}

func (s *Server) saveAISettings(ctx context.Context, a AISettings) error {
	return s.store.SaveSettings(ctx, aiPrefix, map[string]string{
		"preset":   a.Preset.ID,
		"base_url": a.BaseURL,
		"model":    a.Model,
		"api_key":  a.APIKey,
	})
}

// provider cria o provedor de IA com a configuração atual.
func (s *Server) provider(ctx context.Context) (llm.Provider, AISettings, error) {
	a, err := s.aiSettings(ctx)
	if err != nil {
		return nil, a, err
	}
	p, err := llm.New(a.Config)
	return p, a, err
}

// AIStatus resume se a IA está pronta para uso, para mostrar avisos na tela.
type AIStatus struct {
	Ready        bool
	Problem      string   // mensagem para o usuário quando não está pronta
	OllamaOff    bool     // Ollama não instalado/aberto
	ModelMissing bool     // modelo ainda não baixado no Ollama
	Installed    []string // modelos já baixados no Ollama
	Alternatives []string // com o modelo faltando: os de conversa já baixados (usar sem baixar nada)
}

func (s *Server) aiStatus(ctx context.Context, a AISettings) AIStatus {
	p, err := llm.New(a.Config)
	if err != nil {
		return AIStatus{Problem: err.Error()}
	}
	ollama, ok := p.(*llm.Ollama)
	if !ok {
		return AIStatus{Ready: true} // provedores na nuvem só são verificados no teste
	}
	models, err := ollama.Models(ctx)
	if err != nil {
		return AIStatus{OllamaOff: true, Problem: err.Error()}
	}
	st := AIStatus{Installed: models}
	if has, _ := ollama.HasModel(ctx); !has {
		st.ModelMissing = true
		st.Problem = "O modelo " + a.Model + " ainda não foi baixado."
		st.Alternatives = llm.PickModels(models, nil)
		return st
	}
	st.Ready = true
	return st
}
