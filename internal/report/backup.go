package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/domain"
)

// Backup portátil: todos os registros (categorias, lançamentos, dívidas,
// preferências e notas) num JSON. Funciona em qualquer aparelho, inclusive
// sem conexão, e restaura em qualquer outro.

const (
	backupApp     = "finan_rich"
	backupVersion = 1
)

type backupFile struct {
	App        string         `json:"app"`
	Version    int            `json:"version"`
	ExportedAt string         `json:"exportedAt"`
	Counts     map[string]int `json:"counts"`
	Records    []Record       `json:"records"`
}

// Record é um registro dentro do backup (o mesmo formato da sincronização).
type Record struct {
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
}

// Backup grava os registros não apagados.
func Backup(l domain.Ledger, now time.Time) ([]byte, error) {
	f := backupFile{App: backupApp, Version: backupVersion, ExportedAt: now.UTC().Format(time.RFC3339), Counts: map[string]int{}}
	add := func(typ string, v any, deleted bool) error {
		if deleted {
			return nil
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		f.Records = append(f.Records, Record{Type: typ, Record: raw})
		f.Counts[typ]++
		return nil
	}
	var err error
	for _, r := range l.Categories {
		err = errors.Join(err, add(domain.TypeCategory, r, r.Deleted))
	}
	for _, r := range l.Transactions {
		err = errors.Join(err, add(domain.TypeTransaction, r, r.Deleted))
	}
	for _, r := range l.Debts {
		err = errors.Join(err, add(domain.TypeDebt, r, r.Deleted))
	}
	for _, r := range l.Prefs {
		err = errors.Join(err, add(domain.TypePref, r, r.Deleted))
	}
	for _, r := range l.Notes {
		err = errors.Join(err, add(domain.TypeNote, r, r.Deleted))
	}
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(f, "", " ")
}

// ReadBackup confere um arquivo de backup e devolve os registros validados
// (com as mesmas regras de quando são salvos). Um registro inválido recusa o
// arquivo todo: melhor avisar do que restaurar pela metade.
func ReadBackup(data []byte) ([]Record, error) {
	var f backupFile
	if err := json.Unmarshal(data, &f); err != nil || f.App != backupApp {
		return nil, errors.New("este arquivo não é um backup do finan_rich")
	}
	if f.Version > backupVersion {
		return nil, errors.New("backup feito por uma versão mais nova do finan_rich; atualize o app para restaurar")
	}
	out := make([]Record, 0, len(f.Records))
	for i, r := range f.Records {
		_, normalized, err := domain.DecodeRecord(r.Type, r.Record)
		if err != nil {
			return nil, fmt.Errorf("registro %d do backup: %w", i+1, err)
		}
		out = append(out, Record{Type: r.Type, Record: normalized})
	}
	return out, nil
}
