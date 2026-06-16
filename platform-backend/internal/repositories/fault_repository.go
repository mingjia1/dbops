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

type FaultTemplateRepository struct {
	db *Database
}

type FaultExecutionRepository struct {
	db *Database
}

func NewFaultTemplateRepository(db *Database) *FaultTemplateRepository {
	return &FaultTemplateRepository{db: db}
}

func NewFaultExecutionRepository(db *Database) *FaultExecutionRepository {
	return &FaultExecutionRepository{db: db}
}

// --- FaultTemplate ---

func (r *FaultTemplateRepository) Create(ctx context.Context, ft *models.FaultTemplate) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	ft.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO fault_templates (id, name, category, description, fault_type, params, duration_sec, severity, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ft.ID, ft.Name, ft.Category, ft.Description, ft.FaultType, ft.Params, ft.DurationSec, ft.Severity, ft.CreatedAt, ft.UpdatedAt)
	return err
}

func (r *FaultTemplateRepository) List(ctx context.Context, category string) ([]models.FaultTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, category, description, fault_type, params, duration_sec, severity, created_at, updated_at
			FROM fault_templates WHERE category = ? ORDER BY name`, category)
	} else {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, category, description, fault_type, params, duration_sec, severity, created_at, updated_at
			FROM fault_templates ORDER BY category, name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.FaultTemplate, 0)
	for rows.Next() {
		var ft models.FaultTemplate
		if err := rows.Scan(&ft.ID, &ft.Name, &ft.Category, &ft.Description, &ft.FaultType,
			&ft.Params, &ft.DurationSec, &ft.Severity, &ft.CreatedAt, &ft.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ft)
	}
	return out, nil
}

func (r *FaultTemplateRepository) GetByID(ctx context.Context, id string) (*models.FaultTemplate, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var ft models.FaultTemplate
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, category, description, fault_type, params, duration_sec, severity, created_at, updated_at
		FROM fault_templates WHERE id = ?`, id).Scan(
		&ft.ID, &ft.Name, &ft.Category, &ft.Description, &ft.FaultType,
		&ft.Params, &ft.DurationSec, &ft.Severity, &ft.CreatedAt, &ft.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fault template not found")
		}
		return nil, err
	}
	return &ft, nil
}

func (r *FaultTemplateRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM fault_templates WHERE id = ?", id)
	return err
}

// --- FaultExecution ---

func (r *FaultExecutionRepository) Create(ctx context.Context, fe *models.FaultExecution) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	fe.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO fault_executions (id, template_id, drill_id, target_type, target_id, fault_type, params, status, result, started_at, completed_at, rollback_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fe.ID, fe.TemplateID, fe.DrillID, fe.TargetType, fe.TargetID, fe.FaultType,
		fe.Params, fe.Status, fe.Result, fe.StartedAt, fe.CompletedAt, fe.RollbackAt, fe.CreatedAt)
	return err
}

func (r *FaultExecutionRepository) Update(ctx context.Context, fe *models.FaultExecution) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		UPDATE fault_executions SET status = ?, result = ?, completed_at = ?, rollback_at = ? WHERE id = ?`,
		fe.Status, fe.Result, fe.CompletedAt, fe.RollbackAt, fe.ID)
	return err
}

func (r *FaultExecutionRepository) List(ctx context.Context, drillID string) ([]models.FaultExecution, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rows *sql.Rows
	var err error
	if drillID != "" {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, template_id, drill_id, target_type, target_id, fault_type, params, status, result, started_at, completed_at, rollback_at, created_at
			FROM fault_executions WHERE drill_id = ? ORDER BY started_at DESC`, drillID)
	} else {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, template_id, drill_id, target_type, target_id, fault_type, params, status, result, started_at, completed_at, rollback_at, created_at
			FROM fault_executions ORDER BY started_at DESC LIMIT 100`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.FaultExecution, 0)
	for rows.Next() {
		var fe models.FaultExecution
		var completedAt, rollbackAt sql.NullTime
		if err := rows.Scan(&fe.ID, &fe.TemplateID, &fe.DrillID, &fe.TargetType, &fe.TargetID,
			&fe.FaultType, &fe.Params, &fe.Status, &fe.Result, &fe.StartedAt,
			&completedAt, &rollbackAt, &fe.CreatedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			fe.CompletedAt = &completedAt.Time
		}
		if rollbackAt.Valid {
			fe.RollbackAt = &rollbackAt.Time
		}
		out = append(out, fe)
	}
	return out, nil
}

func (r *FaultExecutionRepository) GetByID(ctx context.Context, id string) (*models.FaultExecution, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var fe models.FaultExecution
	var completedAt, rollbackAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, template_id, drill_id, target_type, target_id, fault_type, params, status, result, started_at, completed_at, rollback_at, created_at
		FROM fault_executions WHERE id = ?`, id).Scan(
		&fe.ID, &fe.TemplateID, &fe.DrillID, &fe.TargetType, &fe.TargetID,
		&fe.FaultType, &fe.Params, &fe.Status, &fe.Result, &fe.StartedAt,
		&completedAt, &rollbackAt, &fe.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fault execution not found")
		}
		return nil, err
	}
	if completedAt.Valid {
		fe.CompletedAt = &completedAt.Time
	}
	if rollbackAt.Valid {
		fe.RollbackAt = &rollbackAt.Time
	}
	return &fe, nil
}

var _ = time.Now
