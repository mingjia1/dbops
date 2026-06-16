package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type KeyVersionRepository struct {
	db *Database
}

func NewKeyVersionRepository(db *Database) *KeyVersionRepository {
	return &KeyVersionRepository{db: db}
}

func (r *KeyVersionRepository) Create(ctx context.Context, kv *models.KeyVersion) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if kv.ID == "" {
		kv.ID = uuid.New().String()
	}
	query := `INSERT INTO key_versions (id, key_digest, version, created_at, note) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, kv.ID, kv.KeyDigest, kv.Version, kv.CreatedAt, kv.Note)
	if err != nil {
		return fmt.Errorf("failed to create key version: %w", err)
	}
	return nil
}

func (r *KeyVersionRepository) GetLatest(ctx context.Context) (*models.KeyVersion, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	kv := &models.KeyVersion{}
	err := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, key_digest, version, created_at, note FROM key_versions ORDER BY version DESC LIMIT 1`,
	).Scan(&kv.ID, &kv.KeyDigest, &kv.Version, &kv.CreatedAt, &kv.Note)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest key version: %w", err)
	}
	return kv, nil
}

func (r *KeyVersionRepository) List(ctx context.Context) ([]models.KeyVersion, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, key_digest, version, created_at, note FROM key_versions ORDER BY version DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list key versions: %w", err)
	}
	defer rows.Close()

	out := make([]models.KeyVersion, 0)
	for rows.Next() {
		var kv models.KeyVersion
		if err := rows.Scan(&kv.ID, &kv.KeyDigest, &kv.Version, &kv.CreatedAt, &kv.Note); err != nil {
			return nil, err
		}
		out = append(out, kv)
	}
	return out, rows.Err()
}

func (r *KeyVersionRepository) GetMaxVersion(ctx context.Context) (int, error) {
	if r.db == nil || r.db.Pool == nil {
		return 0, nil
	}
	var maxVer int
	err := r.db.Pool.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM key_versions`).Scan(&maxVer)
	if err != nil {
		return 0, fmt.Errorf("failed to get max version: %w", err)
	}
	return maxVer, nil
}

// GetAllEncryptedData returns all encrypted passwords and SSH credentials for re-encryption.
type EncryptedRecord struct {
	Table      string // "instance_connections" or "hosts"
	RowID      string
	ColumnName string // "password_encrypted" or "ssh_credential"
	Value      string
}

func (r *KeyVersionRepository) GetAllEncryptedData(ctx context.Context) ([]EncryptedRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	out := make([]EncryptedRecord, 0)

	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, password_encrypted FROM instance_connections WHERE password_encrypted != ''`)
	if err != nil {
		return nil, fmt.Errorf("failed to query instance_connections: %w", err)
	}
	for rows.Next() {
		var id, val string
		if err := rows.Scan(&id, &val); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, EncryptedRecord{Table: "instance_connections", RowID: id, ColumnName: "password_encrypted", Value: val})
	}
	rows.Close()

	hostRows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, ssh_credential FROM hosts WHERE ssh_credential != ''`)
	if err != nil {
		return nil, fmt.Errorf("failed to query hosts: %w", err)
	}
	for hostRows.Next() {
		var id, val string
		if err := hostRows.Scan(&id, &val); err != nil {
			hostRows.Close()
			return nil, err
		}
		out = append(out, EncryptedRecord{Table: "hosts", RowID: id, ColumnName: "ssh_credential", Value: val})
	}
	hostRows.Close()

	return out, nil
}

func (r *KeyVersionRepository) UpdateEncryptedValue(ctx context.Context, table, rowID, column, newValue string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	q := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = ?`, table, column)
	_, err := r.db.Pool.ExecContext(ctx, q, newValue, rowID)
	if err != nil {
		return fmt.Errorf("failed to update %s.%s: %w", table, column, err)
	}
	return nil
}
