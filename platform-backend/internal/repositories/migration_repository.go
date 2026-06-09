package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type MigrationRepository struct {
	db *Database

	mu    sync.RWMutex
	memDB map[string]*models.MigrationTask
}

func NewMigrationRepository(db *Database) *MigrationRepository {
	return &MigrationRepository{
		db:    db,
		memDB: make(map[string]*models.MigrationTask),
	}
}

func (r *MigrationRepository) Create(ctx context.Context, task *models.MigrationTask) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	return r.createInDB(ctx, task)
}

func (r *MigrationRepository) createInMemory(task *models.MigrationTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	if task.Status == "" {
		task.Status = models.MigrationStatusPending
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	r.memDB[task.ID] = task
	return nil
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

func (r *MigrationRepository) getByIDInMemory(id string) (*models.MigrationTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.memDB[id]
	if !ok {
		return nil, fmt.Errorf("migration task not found")
	}
	copy := *task
	return &copy, nil
}

func (r *MigrationRepository) getByIDInDB(ctx context.Context, id string) (*models.MigrationTask, error) {
	query := `SELECT id, name, source_instance_id, target_instance_id, strategy, status, progress, config, error, created_at, updated_at FROM migration_tasks WHERE id = ?`
	task := &models.MigrationTask{}
	var startedAt, completedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(&task.ID, &task.Name, &task.SourceInstanceID, &task.TargetInstanceID, &task.Strategy, &task.Status, &task.Progress, &task.Config, &task.Error, &task.CreatedAt, &task.UpdatedAt)
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
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `UPDATE migration_tasks SET status = ?, progress = ?, updated_at = ? WHERE id = ?`, status, progress, time.Now(), id)
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

func (r *MigrationRepository) listInMemory() ([]models.MigrationTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]models.MigrationTask, 0, len(r.memDB))
	for _, t := range r.memDB {
		result = append(result, *t)
	}
	return result, nil
}

func (r *MigrationRepository) listInDB(ctx context.Context) ([]models.MigrationTask, error) {
	rows, err := r.db.Pool.QueryContext(ctx, `SELECT id, name, source_instance_id, target_instance_id, strategy, status, progress, config, error, created_at, updated_at FROM migration_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.MigrationTask, 0)
	for rows.Next() {
		var t models.MigrationTask
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.SourceInstanceID, &t.TargetInstanceID, &t.Strategy, &t.Status, &t.Progress, &t.Config, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
