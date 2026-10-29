package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

func (s *Store) ListChat(ctx context.Context) ([]ChatMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, role, content, proposals, agent, numbers FROM chat_messages ORDER BY id`)
	return collect(rows, err, func(r *sql.Rows) (ChatMessage, error) {
		var m ChatMessage
		var proposals string
		err := r.Scan(&m.ID, &m.Role, &m.Content, &proposals, &m.Agent, &m.Numbers)
		if proposals != "" {
			m.Proposals = json.RawMessage(proposals)
		}
		return m, err
	})
}

// AppendChatTurn grava a pergunta e a resposta juntas, para o histórico nunca
// ficar com uma pergunta sem resposta. proposals (JSON, opcional) são as
// mudanças que o consultor propôs nessa resposta; agent, quem do time respondeu;
// numbers, as contas do app mostradas antes da resposta.
func (s *Store) AppendChatTurn(ctx context.Context, question, answer, agent string, proposals json.RawMessage, numbers string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chat_messages (role, content) VALUES (?, ?)`, RoleUser, question); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO chat_messages (role, content, proposals, agent, numbers) VALUES (?, ?, ?, ?, ?)`, RoleAssistant, answer, string(proposals), agent, numbers)
		return err
	})
}

func (s *Store) ClearChat(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chat_messages`)
	return err
}
