package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Device é um aparelho pareado (app do celular).
type Device struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	LastSeen  time.Time // zero se nunca sincronizou
}

func (s *Store) CreateDevice(ctx context.Context, name, tokenHash string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO devices (name, token_hash, created_at) VALUES (?, ?, ?)`,
		name, tokenHash, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeviceByToken devolve o aparelho dono do token (ok=false se não existe) e
// registra que ele foi visto agora.
func (s *Store) DeviceByToken(ctx context.Context, tokenHash string, now time.Time) (Device, bool, error) {
	var d Device
	err := s.db.QueryRowContext(ctx, `UPDATE devices SET last_seen_at = ? WHERE token_hash = ? RETURNING id, name`,
		now.UTC().Format(timeLayout), tokenHash).Scan(&d.ID, &d.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, false, nil
	}
	return d, err == nil, err
}

func (s *Store) Devices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at, COALESCE(last_seen_at, '') FROM devices ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Device
	for rows.Next() {
		var d Device
		var created, seen string
		if err := rows.Scan(&d.ID, &d.Name, &created, &seen); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(timeLayout, created)
		d.LastSeen, _ = time.Parse(timeLayout, seen)
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) DeleteDevice(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id)
	return err
}

// DeleteAllDevices desconecta todos os apps pareados (ao trocar a senha).
func (s *Store) DeleteAllDevices(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM devices`)
	return err
}
