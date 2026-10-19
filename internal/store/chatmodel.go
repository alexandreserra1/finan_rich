package store

import "encoding/json"

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatMessage é uma mensagem da conversa com o consultor (fica só no servidor,
// pois a IA roda lá).
type ChatMessage struct {
	ID      int64
	Role    string // RoleUser ou RoleAssistant
	Content string
	// Proposals: mudanças propostas pelo consultor nesta resposta (JSON, ou vazio).
	Proposals json.RawMessage
}
