package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Tipos de registro sincronizados (atalhos para os do domínio).
const (
	TypeCategory    = domain.TypeCategory
	TypeTransaction = domain.TypeTransaction
	TypeDebt        = domain.TypeDebt
	TypePref        = domain.TypePref
	TypeNote        = domain.TypeNote
	TypeGoal        = domain.TypeGoal
	TypeRecurring   = domain.TypeRecurring
)

// Change é um registro enviado ou recebido na sincronização.
type Change struct {
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"` // o objeto do domínio, em JSON
}

// ErrInvalidChange indica um registro com tipo ou conteúdo inválido.
var ErrInvalidChange = domain.ErrInvalidRecord

func decodeChange(c Change) (domain.Meta, []byte, error) {
	meta, data, err := domain.DecodeRecord(c.Type, c.Record)
	if err != nil {
		return meta, nil, fmt.Errorf("%s %s: %w", c.Type, meta.ID, err)
	}
	return meta, data, nil
}

// ApplyChanges grava as alterações vindas de um aparelho. Em conflito com o que
// já existe, vence a alteração mais recente (updatedAt maior); em empate, a de
// maior conteúdo em ordem alfabética, para todos os aparelhos chegarem ao mesmo
// resultado. Devolve quantas foram aceitas.
func (s *Store) ApplyChanges(ctx context.Context, changes []Change) (int, error) {
	type decoded struct {
		typ  string
		meta domain.Meta
		data []byte
	}
	list := make([]decoded, 0, len(changes))
	for _, c := range changes {
		meta, data, err := decodeChange(c)
		if err != nil {
			return 0, err
		}
		list = append(list, decoded{c.Type, meta, data})
	}

	applied := 0
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		aliases, err := defaultAliases(ctx, tx, changes)
		if err != nil {
			return err
		}
		for i, d := range list {
			if len(aliases) > 0 {
				if list[i].meta, list[i].data, err = remapDefault(changes[i].Type, d.meta, d.data, aliases); err != nil {
					return err
				}
				d = list[i]
			}
			var curUpdated int64
			var curData string
			err := tx.QueryRowContext(ctx, `SELECT updated_at, data FROM records WHERE type = ? AND id = ?`, d.typ, d.meta.ID).Scan(&curUpdated, &curData)
			switch {
			case errors.Is(err, sql.ErrNoRows):
			case err != nil:
				return err
			case d.meta.UpdatedAt < curUpdated, d.meta.UpdatedAt == curUpdated && string(d.data) <= curData:
				continue // o que já está salvo é mais novo (ou igual)
			}
			rev, err := nextRev(ctx, tx)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO records (type, id, data, updated_at, deleted, rev) VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (type, id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at,
				deleted = excluded.deleted, rev = excluded.rev`,
				d.typ, d.meta.ID, string(d.data), d.meta.UpdatedAt, d.meta.Deleted, rev); err != nil {
				return err
			}
			applied++
		}
		return nil
	})
	return applied, err
}

func nextRev(ctx context.Context, tx *sql.Tx) (int64, error) {
	var rev int64
	err := tx.QueryRowContext(ctx, `UPDATE sync_state SET rev = rev + 1 WHERE id = 1 RETURNING rev`).Scan(&rev)
	return rev, err
}

// ChangesSince devolve os registros alterados depois da revisão informada, em
// ordem, até limit itens, e a revisão do último devolvido (o próximo cursor).
func (s *Store) ChangesSince(ctx context.Context, since int64, limit int) ([]Change, int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type, data, rev FROM records WHERE rev > ? ORDER BY rev LIMIT ?`, since, limit)
	type row struct {
		c   Change
		rev int64
	}
	list, err := collect(rows, err, func(r *sql.Rows) (row, error) {
		var x row
		var data string
		err := r.Scan(&x.c.Type, &data, &x.rev)
		x.c.Record = json.RawMessage(data)
		return x, err
	})
	if err != nil {
		return nil, since, err
	}
	cursor := since
	changes := make([]Change, len(list))
	for i, x := range list {
		changes[i] = x.c
		cursor = x.rev
	}
	return changes, cursor, nil
}

// CurrentRev é a revisão mais recente do servidor.
func (s *Store) CurrentRev(ctx context.Context) (int64, error) {
	var rev int64
	err := s.db.QueryRowContext(ctx, `SELECT rev FROM sync_state WHERE id = 1`).Scan(&rev)
	return rev, err
}

// Ledger carrega todos os registros (inclusive apagados, que as regras ignoram).
func (s *Store) Ledger(ctx context.Context) (domain.Ledger, error) {
	var l domain.Ledger
	rows, err := s.db.QueryContext(ctx, `SELECT type, data FROM records`)
	if err != nil {
		return l, err
	}
	defer rows.Close()
	for rows.Next() {
		var typ, data string
		if err := rows.Scan(&typ, &data); err != nil {
			return l, err
		}
		var target any
		switch typ {
		case TypeCategory:
			l.Categories = append(l.Categories, domain.Category{})
			target = &l.Categories[len(l.Categories)-1]
		case TypeTransaction:
			l.Transactions = append(l.Transactions, domain.Transaction{})
			target = &l.Transactions[len(l.Transactions)-1]
		case TypeDebt:
			l.Debts = append(l.Debts, domain.Debt{})
			target = &l.Debts[len(l.Debts)-1]
		case TypePref:
			l.Prefs = append(l.Prefs, domain.Pref{})
			target = &l.Prefs[len(l.Prefs)-1]
		case TypeNote:
			l.Notes = append(l.Notes, domain.Note{})
			target = &l.Notes[len(l.Notes)-1]
		case TypeGoal:
			l.Goals = append(l.Goals, domain.Goal{})
			target = &l.Goals[len(l.Goals)-1]
		case TypeRecurring:
			l.Recurring = append(l.Recurring, domain.Recurring{})
			target = &l.Recurring[len(l.Recurring)-1]
		default:
			continue
		}
		if err := json.Unmarshal([]byte(data), target); err != nil {
			return l, fmt.Errorf("registro %s corrompido: %w", typ, err)
		}
	}
	return l, rows.Err()
}

// Categorias padrão de um celular usado sozinho (IDs "padrao-…", criadas no
// aparelho antes de ter computador) se juntam às do computador com o mesmo
// nome, em vez de duplicar: a do celular vira apagada e os lançamentos, metas e
// recorrentes que apontavam para ela passam a apontar para a do computador. A
// versão corrigida volta ao celular na sincronização (updatedAt + 1).

// defaultAliases devolve "padrao-x" → ID da categoria do computador de mesmo nome.
func defaultAliases(ctx context.Context, tx *sql.Tx, changes []Change) (map[string]string, error) {
	names := map[string]string{} // padrao-x → nome
	for _, c := range changes {
		var r struct {
			ID, Name, CategoryID string
		}
		json.Unmarshal(c.Record, &r)
		switch {
		case c.Type == TypeCategory && domain.IsDefaultCategoryID(r.ID):
			names[r.ID] = r.Name
		case domain.IsDefaultCategoryID(r.CategoryID) && names[r.CategoryID] == "":
			names[r.CategoryID] = "" // descobre o nome pelo que já está salvo
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, data FROM records WHERE type = ?`, TypeCategory)
	cats, err := collect(rows, err, func(r *sql.Rows) (domain.Category, error) {
		var id, data string
		var c domain.Category
		if err := r.Scan(&id, &data); err != nil {
			return c, err
		}
		return c, json.Unmarshal([]byte(data), &c)
	})
	if err != nil {
		return nil, err
	}
	aliases := map[string]string{}
	for id, name := range names {
		if name == "" {
			for _, c := range cats {
				if c.ID == id {
					name = c.Name
				}
			}
		}
		for _, c := range cats { // sem par no computador: a do celular fica como está
			if name != "" && !c.Deleted && !domain.IsDefaultCategoryID(c.ID) && strings.EqualFold(strings.TrimSpace(c.Name), strings.TrimSpace(name)) {
				aliases[id] = c.ID
				break
			}
		}
	}
	return aliases, nil
}

// remapDefault aplica os apelidos a um registro recebido.
func remapDefault(typ string, meta domain.Meta, data []byte, aliases map[string]string) (domain.Meta, []byte, error) {
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		return meta, data, err
	}
	changed := false
	if typ == TypeCategory && aliases[meta.ID] != "" && !meta.Deleted {
		rec["deleted"], changed = true, true
	}
	if id, _ := rec["categoryId"].(string); aliases[id] != "" {
		rec["categoryId"], changed = aliases[id], true
	}
	if !changed {
		return meta, data, nil
	}
	rec["updatedAt"] = meta.UpdatedAt + 1
	raw, err := json.Marshal(rec)
	if err != nil {
		return meta, data, err
	}
	return domain.DecodeRecord(typ, raw)
}
