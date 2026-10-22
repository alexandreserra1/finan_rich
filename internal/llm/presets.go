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
	KeySteps    []string // passo a passo para pegar a chave (até copiar; colar é com a tela)
	Models      []string // sugestões de modelo
}

// Presets em ordem de recomendação. Os nomes de modelo são sugestões e podem
// ser trocados livremente na tela de configurações.
var Presets = []Preset{
	{
		ID: "ollama", Name: "Ollama (no seu computador)", Kind: KindOllama,
		Description: "Grátis, sem chave e privado: seus dados não saem do PC. Precisa do Ollama instalado, ~7 GB para o modelo e de preferência 16 GB de RAM.",
		BaseURL:     DefaultOllamaURL, Model: "qwen3.5:9b",
		Models: []string{"qwen3.5:9b", "qwen3.5:4b", "qwen3.5:2b", "qwen3:4b"},
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
		KeySteps: []string{
			"Entre com sua conta Google (é grátis, não pede cartão).",
			"Toque em \"Create API Key\", dê um nome (por exemplo, finan_rich) e confirme.",
			"Copie a chave (começa com gsk_). Ela só aparece uma vez.",
		},
		Models: []string{"qwen/qwen3-32b", "llama-3.3-70b-versatile", "openai/gpt-oss-120b"},
	},
	{
		ID: "gemini", Name: "Google Gemini (nuvem, grátis)", Kind: KindOpenAI,
		Description: "Plano gratuito com limite diário. Atenção: no plano grátis o Google pode usar o que é enviado para melhorar os produtos dele (o finan_rich manda tudo anonimizado). Precisa de uma chave grátis do Google AI Studio.",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-3.5-flash", NeedsKey: true,
		KeyURL: "https://aistudio.google.com/apikey",
		KeySteps: []string{
			"Entre com sua conta Google (é grátis, não pede cartão).",
			"Aceite os termos, se pedir, e toque em \"Create API key\" (Criar chave de API).",
			"Copie a chave que aparecer.",
		},
		Models: []string{"gemini-3.5-flash", "gemini-flash-latest", "gemini-flash-lite-latest"},
	},
	{
		ID: "openrouter", Name: "OpenRouter (nuvem, modelos grátis)", Kind: KindOpenAI,
		Description: "Dá acesso a vários modelos abertos; os terminados em \":free\" não custam nada. Precisa de chave.",
		BaseURL:     "https://openrouter.ai/api/v1", Model: "deepseek/deepseek-chat-v3.1:free", NeedsKey: true,
		KeyURL: "https://openrouter.ai/keys",
		KeySteps: []string{
			"Crie uma conta (pode ser com o Google).",
			"Toque em \"Create API Key\", dê um nome e confirme.",
			"Copie a chave (começa com sk-or-). Use só modelos terminados em \":free\" para não pagar.",
		},
		Models: []string{"deepseek/deepseek-chat-v3.1:free", "qwen/qwen3-235b-a22b:free"},
	},
	{
		ID: "deepseek", Name: "DeepSeek (nuvem, muito barato)", Kind: KindOpenAI,
		Description: "API oficial da DeepSeek. Paga, mas custa centavos por conversa.",
		BaseURL:     "https://api.deepseek.com", Model: "deepseek-chat", NeedsKey: true,
		KeyURL: "https://platform.deepseek.com/api_keys",
		KeySteps: []string{
			"Crie uma conta e coloque crédito (esta é paga, mas barata).",
			"Toque em \"Create new API key\", dê um nome e confirme.",
			"Copie a chave (começa com sk-). Ela só aparece uma vez.",
		},
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
// FreeProxyURL é o endereço do intermediário grátis do finan_rich (cmd/finanproxy,
// no Cloud Run; ver docs/IA-GRATIS.md). Não é segredo: a chave fica no servidor.
// Vazio: a opção não aparece.
var FreeProxyURL = ""

// FreeID é o preset do intermediário grátis.
const FreeID = "finan"

// All devolve os presets oferecidos; o grátis do finan_rich vem primeiro quando existe.
func All() []Preset {
	if FreeProxyURL == "" {
		return Presets
	}
	free := Preset{
		ID: FreeID, Name: "finan_rich grátis (sem chave)", Kind: KindOpenAI, BaseURL: FreeProxyURL, Model: "auto",
		Description: "Não precisa de chave nem de conta: o finan_rich usa modelos abertos no Groq, que não usa os dados para treinar. Vai tudo com apelidos. Tem limite de perguntas por dia em cada aparelho; se acabar, use sua própria chave grátis.",
	}
	return append([]Preset{free}, Presets...)
}

func PresetByID(id string) Preset {
	for _, p := range All() {
		if p.ID == id {
			return p
		}
	}
	return Presets[0]
}
