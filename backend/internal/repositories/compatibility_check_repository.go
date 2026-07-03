package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type CompatibilityCheckRepository struct {
	db *Database
}

func NewCompatibilityCheckRepository(db *Database) *CompatibilityCheckRepository {
	return &CompatibilityCheckRepository{db: db}
}

func (r *CompatibilityCheckRepository) Create(ctx context.Context, check *models.CompatibilityCheck) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if check.CreatedAt.IsZero() {
		check.CreatedAt = time.Now()
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO compatibility_checks (
			id, instance_id, source_version, target_version, check_type, status,
			is_compatible, warning_count, error_count, incompatibility_list,
			checked_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		check.ID, check.InstanceID, check.SourceVersion, check.TargetVersion,
		check.CheckType, check.Status,
		boolToInt(check.IsCompatible), check.WarningCount, check.ErrorCount,
		check.IncompatibilityList,
		check.CheckedAt, check.CreatedAt,
	)
	return err
}

func (r *CompatibilityCheckRepository) GetByID(ctx context.Context, id string) (*models.CompatibilityCheck, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, source_version, target_version, check_type, status,
		       is_compatible, warning_count, error_count, incompatibility_list,
		       checked_at, created_at
		FROM compatibility_checks WHERE id = ?`, id)
	check := &models.CompatibilityCheck{}
	var isCompat int
	if err := row.Scan(
		&check.ID, &check.InstanceID, &check.SourceVersion, &check.TargetVersion,
		&check.CheckType, &check.Status,
		&isCompat, &check.WarningCount, &check.ErrorCount,
		&check.IncompatibilityList,
		&check.CheckedAt, &check.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("compatibility check not found: %s", id)
		}
		return nil, err
	}
	check.IsCompatible = isCompat != 0
	return check, nil
}

func (r *CompatibilityCheckRepository) ListByInstance(ctx context.Context, instanceID string, limit, offset int) ([]*models.CompatibilityCheck, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, instance_id, source_version, target_version, check_type, status,
		       is_compatible, warning_count, error_count, incompatibility_list,
		       checked_at, created_at
		FROM compatibility_checks WHERE instance_id = ?
		ORDER BY checked_at DESC LIMIT ? OFFSET ?`,
		instanceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list compatibility checks: %w", err)
	}
	defer rows.Close()

	var checks []*models.CompatibilityCheck
	for rows.Next() {
		check := &models.CompatibilityCheck{}
		var isCompat int
		if err := rows.Scan(
			&check.ID, &check.InstanceID, &check.SourceVersion, &check.TargetVersion,
			&check.CheckType, &check.Status,
			&isCompat, &check.WarningCount, &check.ErrorCount,
			&check.IncompatibilityList, &check.CheckedAt, &check.CreatedAt,
		); err != nil {
			return nil, err
		}
		check.IsCompatible = isCompat != 0
		checks = append(checks, check)
	}
	return checks, rows.Err()
}


