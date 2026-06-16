package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type MaskingRepository struct {
	db *Database
}

func NewMaskingRepository(db *Database) *MaskingRepository {
	return &MaskingRepository{db: db}
}

func (r *MaskingRepository) Create(ctx context.Context, rule *models.MaskingRule) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = time.Now()
	}
	query := `INSERT INTO masking_rules (id, name, description, field_path, pattern, algorithm, replacement, roles, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.ID, rule.Name, rule.Description, rule.FieldPath, rule.Pattern,
		rule.Algorithm, rule.Replacement, joinStrings(rule.Roles), boolToInt(rule.Enabled),
		rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create masking rule: %w", err)
	}
	return nil
}

func (r *MaskingRepository) GetByID(ctx context.Context, id string) (*models.MaskingRule, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, name, description, field_path, pattern, algorithm, replacement, roles, enabled, created_at, updated_at FROM masking_rules WHERE id = ?`
	rule := &models.MaskingRule{}
	var roles string
	var enabled int
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.FieldPath, &rule.Pattern,
		&rule.Algorithm, &rule.Replacement, &roles, &enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("masking rule not found")
		}
		return nil, fmt.Errorf("failed to get masking rule: %w", err)
	}
	rule.Roles = splitStrings(roles)
	rule.Enabled = intToBool(enabled)
	return rule, nil
}

func (r *MaskingRepository) List(ctx context.Context) ([]models.MaskingRule, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, name, description, field_path, pattern, algorithm, replacement, roles, enabled, created_at, updated_at FROM masking_rules ORDER BY created_at DESC`
	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list masking rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.MaskingRule, 0)
	for rows.Next() {
		var rule models.MaskingRule
		var roles string
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.FieldPath, &rule.Pattern,
			&rule.Algorithm, &rule.Replacement, &roles, &enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rule.Roles = splitStrings(roles)
		rule.Enabled = intToBool(enabled)
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *MaskingRepository) Update(ctx context.Context, rule *models.MaskingRule) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	rule.UpdatedAt = time.Now()
	query := `UPDATE masking_rules SET name=?, description=?, field_path=?, pattern=?, algorithm=?, replacement=?, roles=?, enabled=?, updated_at=? WHERE id=?`
	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.Name, rule.Description, rule.FieldPath, rule.Pattern,
		rule.Algorithm, rule.Replacement, joinStrings(rule.Roles), boolToInt(rule.Enabled),
		rule.UpdatedAt, rule.ID)
	if err != nil {
		return fmt.Errorf("failed to update masking rule: %w", err)
	}
	return nil
}

func (r *MaskingRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM masking_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete masking rule: %w", err)
	}
	return nil
}

func (r *MaskingRepository) ListEnabled(ctx context.Context) ([]models.MaskingRule, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, name, description, field_path, pattern, algorithm, replacement, roles, enabled, created_at, updated_at FROM masking_rules WHERE enabled = 1 ORDER BY created_at DESC`
	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled masking rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.MaskingRule, 0)
	for rows.Next() {
		var rule models.MaskingRule
		var roles string
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.FieldPath, &rule.Pattern,
			&rule.Algorithm, &rule.Replacement, &roles, &enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rule.Roles = splitStrings(roles)
		rule.Enabled = intToBool(enabled)
		rules = append(rules, rule)
	}
	return rules, nil
}
