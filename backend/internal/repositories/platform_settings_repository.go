package repositories

import (
	"context"
	"fmt"
)

type PlatformSettingsRepository struct {
	db *Database
}

func NewPlatformSettingsRepository(db *Database) *PlatformSettingsRepository {
	return &PlatformSettingsRepository{db: db}
}

func (r *PlatformSettingsRepository) Get(ctx context.Context, key string) (string, error) {
	if r.db == nil || r.db.Pool == nil {
		return "", fmt.Errorf("database not initialized")
	}
	var value string
	err := r.db.Pool.QueryRowContext(ctx, "SELECT value FROM platform_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *PlatformSettingsRepository) Set(ctx context.Context, key, value string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx,
		`INSERT INTO platform_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value)
	return err
}

func (r *PlatformSettingsRepository) Delete(ctx context.Context, key string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM platform_settings WHERE key = ?", key)
	return err
}

func (r *PlatformSettingsRepository) List(ctx context.Context) (map[string]string, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx, "SELECT key, value FROM platform_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		result[key] = value
	}
	return result, nil
}
