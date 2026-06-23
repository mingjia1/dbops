package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type LicenseRepository struct {
	db *Database
}

func NewLicenseRepository(db *Database) *LicenseRepository {
	return &LicenseRepository{db: db}
}

func (r *LicenseRepository) Save(ctx context.Context, license *models.License) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if license.ID == "" {
		license.ID = uuid.New().String()
	}
	q := `INSERT INTO licenses (id, tier, license_key, issued_to, issued_at, expires_at, max_nodes, license_json, active, created_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, q, license.ID, license.Tier, license.LicenseKey, license.IssuedTo,
		license.IssuedAt, license.ExpiresAt, license.MaxNodes, license.LicenseJSON, license.Active, license.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to save license: %w", err)
	}
	return nil
}

func (r *LicenseRepository) GetActive(ctx context.Context) (*models.License, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	l := &models.License{}
	q := `SELECT id, tier, license_key, issued_to, issued_at, expires_at, max_nodes, license_json, active, created_at
		  FROM licenses WHERE active = 1 ORDER BY created_at DESC LIMIT 1`
	err := r.db.Pool.QueryRowContext(ctx, q).Scan(&l.ID, &l.Tier, &l.LicenseKey, &l.IssuedTo,
		&l.IssuedAt, &l.ExpiresAt, &l.MaxNodes, &l.LicenseJSON, &l.Active, &l.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active license: %w", err)
	}
	return l, nil
}

func (r *LicenseRepository) DeactivateAll(ctx context.Context) error {
	if r.db == nil || r.db.Pool == nil {
		return nil
	}
	_, err := r.db.Pool.ExecContext(ctx, `UPDATE licenses SET active = 0`)
	return err
}

func (r *LicenseRepository) List(ctx context.Context) ([]models.License, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, tier, license_key, issued_to, issued_at, expires_at, max_nodes, license_json, active, created_at
		 FROM licenses ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list licenses: %w", err)
	}
	defer rows.Close()

	out := make([]models.License, 0)
	for rows.Next() {
		var l models.License
		if err := rows.Scan(&l.ID, &l.Tier, &l.LicenseKey, &l.IssuedTo,
			&l.IssuedAt, &l.ExpiresAt, &l.MaxNodes, &l.LicenseJSON, &l.Active, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
