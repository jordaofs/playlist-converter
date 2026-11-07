package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"playlistconverter/internal/domain"
)

func (s *Store) Save(ctx context.Context, c domain.Conversion) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO conversions(id,principal_id,source_identity,target_identity,data,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, c.ID, c.PrincipalID, c.SourceIdentity, c.TargetIdentity, data, c.UpdatedAt.Unix())
	return err
}
func (s *Store) Get(ctx context.Context, id string) (domain.Conversion, error) {
	var c domain.Conversion
	var data []byte
	var principal, source, target string
	err := s.DB.QueryRowContext(ctx, `SELECT data,principal_id,source_identity,target_identity FROM conversions WHERE id=?`, id).Scan(&data, &principal, &source, &target)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	c.PrincipalID = principal
	c.SourceIdentity = source
	c.TargetIdentity = target
	return c, nil
}
func (s *Store) List(ctx context.Context, principal string) ([]domain.Conversion, error) {
	query := `SELECT data,principal_id,source_identity,target_identity FROM conversions`
	var args []any
	if principal != "" {
		query += ` WHERE principal_id=?`
		args = append(args, principal)
	}
	query += ` ORDER BY updated_at DESC,id`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Conversion{}
	for rows.Next() {
		var c domain.Conversion
		var data []byte
		var p, a, b string
		if err = rows.Scan(&data, &p, &a, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		c.PrincipalID = p
		c.SourceIdentity = a
		c.TargetIdentity = b
		out = append(out, c)
	}
	return out, rows.Err()
}

type Operation struct {
	Kind     string
	Position int
	TargetID string
}

func (s *Store) Intent(ctx context.Context, id string, op Operation) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO operations(conversion_id,kind,position,target_id) VALUES(?,?,?,?)`, id, op.Kind, op.Position, op.TargetID)
	return err
}
func (s *Store) Operation(ctx context.Context, id string) (Operation, error) {
	var o Operation
	err := s.DB.QueryRowContext(ctx, `SELECT kind,position,target_id FROM operations WHERE conversion_id=?`, id).Scan(&o.Kind, &o.Position, &o.TargetID)
	return o, err
}
func (s *Store) ClearIntent(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM operations WHERE conversion_id=?`, id)
	return err
}
func (s *Store) Idempotency(ctx context.Context, p, scope, key string) (payload, id string, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT payload,conversion_id FROM idempotency WHERE principal_id=? AND scope=? AND key=?`, p, scope, key).Scan(&payload, &id)
	return
}

// SaveKey atomically stores a job and its submission idempotency record.
func (s *Store) SaveKey(ctx context.Context, c domain.Conversion, scope, key, payload string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO conversions(id,principal_id,source_identity,target_identity,data,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,updated_at=excluded.updated_at`, c.ID, c.PrincipalID, c.SourceIdentity, c.TargetIdentity, data, c.UpdatedAt.Unix())
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency(principal_id,scope,key,payload,conversion_id) VALUES(?,?,?,?,?)`, c.PrincipalID, scope, key, payload, c.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func IsMissing(err error) bool { return err == sql.ErrNoRows }
