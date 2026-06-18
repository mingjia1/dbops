package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type MigrationRepository struct {
	db *Database
}

func NewMigrationRepository(db *Database) *MigrationRepository {
	return &MigrationRepository{
		db: db,
	}
}

func (r *MigrationRepository) Create(ctx context.Context, task *models.MigrationTask) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	return r.createInDB(ctx, task)
}

func (r *MigrationRepository) createInDB(ctx context.Context, task *models.MigrationTask) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	query := `INSERT INTO migration_tasks (id, name, source_instance_id, target_instance_id, strategy, status, progress, config, error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, task.ID, task.Name, task.SourceInstanceID, task.TargetInstanceID, task.Strategy, task.Status, task.Progress, task.Config, task.Error, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create migration task: %w", err)
	}
	return nil
}

func (r *MigrationRepository) GetByID(ctx context.Context, id string) (*models.MigrationTask, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	return r.getByIDInDB(ctx, id)
}

func (r *MigrationRepository) getByIDInDB(ctx context.Context, id string) (*models.MigrationTask, error) {
	query := `SELECT id, name, source_instance_id, target_instance_id, strategy, status, progress, started_at, completed_at, config, error, created_at, updated_at FROM migration_tasks WHERE id = ?`
	task := &models.MigrationTask{}
	var startedAt, completedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(&task.ID, &task.Name, &task.SourceInstanceID, &task.TargetInstanceID, &task.Strategy, &task.Status, &task.Progress, &startedAt, &completedAt, &task.Config, &task.Error, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration task not found")
		}
		return nil, fmt.Errorf("failed to get migration task: %w", err)
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	return task, nil
}

func (r *MigrationRepository) UpdateStatus(ctx context.Context, id string, status models.MigrationStatus, progress int) error {
	return r.updateStatus(ctx, id, status, progress, nil)
}

func (r *MigrationRepository) UpdateStatusWithError(ctx context.Context, id string, status models.MigrationStatus, progress int, errorMsg string) error {
	return r.updateStatus(ctx, id, status, progress, &errorMsg)
}

func (r *MigrationRepository) updateStatus(ctx context.Context, id string, status models.MigrationStatus, progress int, errorMsg *string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	query := `UPDATE migration_tasks SET status = ?, progress = ?, updated_at = ?`
	args := []interface{}{status, progress, now}
	if errorMsg != nil {
		query += `, error = ?`
		args = append(args, *errorMsg)
	}
	if isMigrationStartedStatus(status) {
		query += `, started_at = COALESCE(started_at, ?)`
		args = append(args, now)
	}
	if isMigrationTerminalStatus(status) {
		query += `, completed_at = COALESCE(completed_at, ?)`
		args = append(args, now)
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	res, err := r.db.Pool.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("migration task not found")
	}
	return nil
}

func (r *MigrationRepository) List(ctx context.Context) ([]models.MigrationTask, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	return r.listInDB(ctx)
}

func (r *MigrationRepository) listInDB(ctx context.Context) ([]models.MigrationTask, error) {
	rows, err := r.db.Pool.QueryContext(ctx, `SELECT id, name, source_instance_id, target_instance_id, strategy, status, progress, started_at, completed_at, config, error, created_at, updated_at FROM migration_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.MigrationTask, 0)
	for rows.Next() {
		var t models.MigrationTask
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.SourceInstanceID, &t.TargetInstanceID, &t.Strategy, &t.Status, &t.Progress, &startedAt, &completedAt, &t.Config, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			t.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		result = append(result, t)
	}
	return result, nil
}

func isMigrationStartedStatus(status models.MigrationStatus) bool {
	switch status {
	case models.MigrationStatusPreparing, models.MigrationStatusMigrating, models.MigrationStatusVerifying, models.MigrationStatusSwitching, models.MigrationStatusCompleted, models.MigrationStatusFailed, models.MigrationStatusCancelled:
		return true
	default:
		return false
	}
}

func isMigrationTerminalStatus(status models.MigrationStatus) bool {
	switch status {
	case models.MigrationStatusCompleted, models.MigrationStatusFailed, models.MigrationStatusCancelled:
		return true
	default:
		return false
	}
}
