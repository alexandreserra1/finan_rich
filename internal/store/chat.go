package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListChat(ctx context.Context) ([]ChatMessage, error) {
	rows, err := s.db.Query(ctx, `SELECT id, role, display, content, created_at FROM chat_messages ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ChatMessage, error) {
		var m ChatMessage
		err := r.Scan(&m.ID, &m.Role, &m.Display, &m.Content, &m.CreatedAt)
		return m, err
	})
}

// AppendChatTurn grava a pergunta e a resposta juntas, para o histórico nunca
// ficar com uma pergunta sem resposta.
func (s *Store) AppendChatTurn(ctx context.Context, user, assistant ChatMessage) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		for _, m := range []ChatMessage{user, assistant} {
			if _, err := tx.Exec(ctx, `INSERT INTO chat_messages (role, display, content) VALUES ($1, $2, $3)`,
				m.Role, m.Display, m.Content); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ClearChat(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM chat_messages`)
	return err
}
