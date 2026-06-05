package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type TaskRepository struct {
	db *Database
}

func NewTaskRepository(db *Database) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	task.ID = uuid.New().String()
	task.Status = "pending"

	query := `
		INSERT INTO tasks (id, task_type, instance_id, status, progress, created_at, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		task.ID, task.TaskType, task.InstanceID, task.Status, task.Progress, task.CreatedAt, task.ErrorMessage)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, task_type, instance_id, status, progress, created_at, started_at, completed_at, error_message
		FROM tasks WHERE id = ?
	`

	task := &models.Task{}
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.TaskType, &task.InstanceID, &task.Status, &task.Progress,
		&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.ErrorMessage)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id string, status string, progress int) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `
		UPDATE tasks SET status = ?, progress = ?, updated_at = NOW() WHERE id = ?
	`
	_, err := r.db.Pool.ExecContext(ctx, query, status, progress, id)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	return nil
}

func (r *TaskRepository) AddLog(ctx context.Context, taskLog *models.TaskLog) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	taskLog.ID = uuid.New().String()

	query := `
		INSERT INTO task_logs (id, task_id, log_id, timestamp, level, message, context)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		taskLog.ID, taskLog.TaskID, taskLog.LogID, taskLog.Timestamp, taskLog.Level, taskLog.Message, taskLog.Context)

	if err != nil {
		return fmt.Errorf("failed to add task log: %w", err)
	}

	return nil
}

func (r *TaskRepository) List(ctx context.Context, instanceID string, limit, offset int) ([]models.Task, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, task_type, instance_id, status, progress, created_at, started_at, completed_at, error_message
		FROM tasks WHERE instance_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, instanceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		if err := rows.Scan(&task.ID, &task.TaskType, &task.InstanceID, &task.Status, &task.Progress,
			&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.ErrorMessage); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
