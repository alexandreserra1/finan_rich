package store

import (
	"context"
	"database/sql"
)

func (s *Store) ListChat(ctx context.Context) ([]ChatMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, role, content FROM chat_messages ORDER BY id`)
	return collect(rows, err, func(r *sql.Rows) (ChatMessage, error) {
		var m ChatMessage
		err := r.Scan(&m.ID, &m.Role, &m.Content)
		return m, err
	})
}

// AppendChatTurn grava a pergunta e a resposta juntas, para o histórico nunca
// ficar com uma pergunta sem resposta.
func (s *Store) AppendChatTurn(ctx context.Context, question, answer string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, m := range []ChatMessage{{Role: RoleUser, Content: question}, {Role: RoleAssistant, Content: answer}} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO chat_messages (role, content) VALUES (?, ?)`, m.Role, m.Content); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ClearChat(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chat_messages`)
	return err
}
