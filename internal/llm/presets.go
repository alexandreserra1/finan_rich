package llm

// Preset é uma configuração pronta oferecida na tela de configurações.
type Preset struct {
	ID          string
	Name        string
	Description string
	Kind        string
	BaseURL     string
	Model       string
	NeedsKey    bool
	KeyURL      string   // onde o usuário gera a chave
	Models      []string // sugestões de modelo
}

// Presets em ordem de recomendação. Os nomes de modelo são sugestões e podem
// ser trocados livremente na tela de configurações.
var Presets = []Preset{
	{
		ID: "ollama", Name: "Ollama (no seu computador)", Kind: KindOllama,
		Description: "Grátis, sem chave e privado: seus dados não saem do PC. Precisa do Ollama instalado e de ~3,5 GB para o modelo.",
		BaseURL:     DefaultOllamaURL, Model: "qwen3.5:4b",
		Models: []string{"qwen3.5:4b", "qwen3.5:9b", "qwen3.5:2b", "qwen3:4b"},
	},
	{
		ID: "ollama-cloud", Name: "Ollama Cloud (modelos gigantes, grátis com conta)", Kind: KindOllama,
		Description: "Modelos chineses de ponta (Qwen3.5, Kimi, GLM) rodando nos servidores do Ollama. Plano grátis com limite de uso; precisa do Ollama e de uma conta (\"ollama signin\").",
		BaseURL:     DefaultOllamaURL, Model: "qwen3.5:cloud",
		Models: []string{"qwen3.5:cloud", "kimi-k2.6:cloud", "glm-5.1:cloud", "minimax-m2.7:cloud"},
	},
	{
		ID: "groq", Name: "Groq (nuvem, grátis)", Kind: KindOpenAI,
		Description: "Muito rápido e com plano gratuito. Precisa de uma chave grátis (login com Google).",
		BaseURL:     "https://api.groq.com/openai/v1", Model: "qwen/qwen3-32b", NeedsKey: true,
		KeyURL: "https://console.groq.com/keys",
		Models: []string{"qwen/qwen3-32b", "llama-3.3-70b-versatile", "openai/gpt-oss-120b"},
	},
	{
		ID: "openrouter", Name: "OpenRouter (nuvem, modelos grátis)", Kind: KindOpenAI,
		Description: "Dá acesso a vários modelos abertos; os terminados em \":free\" não custam nada. Precisa de chave.",
		BaseURL:     "https://openrouter.ai/api/v1", Model: "deepseek/deepseek-chat-v3.1:free", NeedsKey: true,
		KeyURL: "https://openrouter.ai/keys",
		Models: []string{"deepseek/deepseek-chat-v3.1:free", "qwen/qwen3-235b-a22b:free"},
	},
	{
		ID: "deepseek", Name: "DeepSeek (nuvem, muito barato)", Kind: KindOpenAI,
		Description: "API oficial da DeepSeek. Paga, mas custa centavos por conversa.",
		BaseURL:     "https://api.deepseek.com", Model: "deepseek-chat", NeedsKey: true,
		KeyURL: "https://platform.deepseek.com/api_keys",
		Models: []string{"deepseek-chat", "deepseek-reasoner"},
	},
	{
		ID: "custom", Name: "Outro compatível com OpenAI", Kind: KindOpenAI,
		Description: "LM Studio, vLLM, OpenAI ou qualquer API no formato /v1/chat/completions.",
		BaseURL:     "http://localhost:1234/v1", Model: "",
	},
	{
		ID: "anthropic", Name: "Anthropic Claude (nuvem, pago)", Kind: KindAnthropic,
		Description: "Modelos Claude, pagos por uso. Precisa de chave da Anthropic.",
		Model:       "claude-opus-5-5", NeedsKey: true, KeyURL: "https://platform.claude.com/",
		Models: []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"},
	},
}

// PresetByID devolve o preset com o id informado (ou o primeiro, o Ollama).
func PresetByID(id string) Preset {
	for _, p := range Presets {
		if p.ID == id {
			return p
		}
	}
	return Presets[0]
}
