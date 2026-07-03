package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type UpgradePlanRepository struct {
	db *Database
}

func NewUpgradePlanRepository(db *Database) *UpgradePlanRepository {
	return &UpgradePlanRepository{db: db}
}

func (r *UpgradePlanRepository) Create(ctx context.Context, plan *models.UpgradePlan) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	now := time.Now()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	if plan.UpdatedAt.IsZero() {
		plan.UpdatedAt = now
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO upgrade_plans (
			id, instance_id, source_version, target_version, strategy, status,
			pre_check_result, upgrade_path, estimated_time, risk_level,
			created_at, updated_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		plan.ID, plan.InstanceID, plan.SourceVersion, plan.TargetVersion, plan.Strategy, plan.Status,
		plan.PreCheckResult, plan.UpgradePath, plan.EstimatedTime, plan.RiskLevel,
		plan.CreatedAt, plan.UpdatedAt, nullTime(plan.StartedAt), nullTime(plan.CompletedAt),
	)
	return err
}

func (r *UpgradePlanRepository) GetByID(ctx context.Context, id string) (*models.UpgradePlan, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, source_version, target_version, strategy, status,
		       pre_check_result, upgrade_path, estimated_time, risk_level,
		       created_at, updated_at, started_at, completed_at
		FROM upgrade_plans WHERE id = ?`, id)
	plan := &models.UpgradePlan{}
	var startedAt, completedAt sql.NullTime
	if err := row.Scan(
		&plan.ID, &plan.InstanceID, &plan.SourceVersion, &plan.TargetVersion, &plan.Strategy, &plan.Status,
		&plan.PreCheckResult, &plan.UpgradePath, &plan.EstimatedTime, &plan.RiskLevel,
		&plan.CreatedAt, &plan.UpdatedAt, &startedAt, &completedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("upgrade plan not found: %s", id)
		}
		return nil, err
	}
	if startedAt.Valid {
		plan.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		plan.CompletedAt = completedAt.Time
	}
	return plan, nil
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
