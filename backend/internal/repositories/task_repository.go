package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
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
	if strings.TrimSpace(task.ID) == "" {
		task.ID = uuid.New().String()
	}
	if strings.TrimSpace(task.Status) == "" {
		task.Status = "pending"
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO tasks (id, task_type, instance_id, status, progress, created_at, started_at, completed_at, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		task.ID, task.TaskType, nullableString(task.InstanceID), task.Status, task.Progress,
		FormatTime(task.CreatedAt), FormatTime(task.StartedAt), FormatTime(task.CompletedAt), task.ErrorMessage)

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
	err := scanTask(r.db.Pool.QueryRowContext(ctx, query, id), task)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	logs, err := r.ListLogs(ctx, id, 200, 0)
	if err == nil {
		task.Logs = logs
	}

	return task, nil
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id string, status string, progress int) error {
	return r.UpdateStatusWithMessage(ctx, id, status, progress, "")
}

func (r *TaskRepository) UpdateStatusWithMessage(ctx context.Context, id string, status string, progress int, message string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	message = strings.TrimSpace(message)
	if message != "" {
		if existing, err := r.GetByID(ctx, id); err == nil && strings.TrimSpace(existing.ErrorMessage) != "" {
			if strings.HasPrefix(existing.ErrorMessage, "plan:") {
				message = existing.ErrorMessage + "\n" + message
			}
		}
	}
	if status == "completed" || status == "success" || status == "failed" || status == "cancelled" || status == "canceled" {
		query := `
			UPDATE tasks
			SET status = ?, progress = ?, completed_at = ?
		`
		args := []interface{}{status, progress, FormatTime(now)}
		if message != "" {
			query += `, error_message = ?`
			args = append(args, message)
		}
		query += ` WHERE id = ?`
		args = append(args, id)
		_, err := r.db.Pool.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}
		return nil
	}
	query := `
		UPDATE tasks
		SET status = ?, progress = ?, started_at = COALESCE(started_at, ?)
		WHERE id = ?
	`
	args := []interface{}{status, progress, FormatTime(now), id}
	if message != "" {
		query = `
			UPDATE tasks
			SET status = ?, progress = ?, started_at = COALESCE(started_at, ?), error_message = ?
			WHERE id = ?
		`
		args = []interface{}{status, progress, FormatTime(now), message, id}
	}
	_, err := r.db.Pool.ExecContext(ctx, query, args...)
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

func (r *TaskRepository) ListLogs(ctx context.Context, taskID string, limit, offset int) ([]models.TaskLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, task_id, log_id, timestamp, level, message, context
		FROM task_logs
		WHERE task_id = ?
		ORDER BY timestamp ASC, id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list task logs: %w", err)
	}
	defer rows.Close()

	logs := []models.TaskLog{}
	for rows.Next() {
		var taskLog models.TaskLog
		if err := rows.Scan(&taskLog.ID, &taskLog.TaskID, &taskLog.LogID, &taskLog.Timestamp, &taskLog.Level, &taskLog.Message, &taskLog.Context); err != nil {
			return nil, err
		}
		logs = append(logs, taskLog)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
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

	return scanTaskRows(rows)
}

func (r *TaskRepository) ListByTypes(ctx context.Context, taskTypes []string, limit, offset int) ([]models.Task, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	filtered := make([]string, 0, len(taskTypes))
	for _, taskType := range taskTypes {
		taskType = strings.TrimSpace(taskType)
		if taskType != "" {
			filtered = append(filtered, taskType)
		}
	}
	if len(filtered) == 0 {
		return []models.Task{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(filtered)), ",")
	query := fmt.Sprintf(`
		SELECT id, task_type, instance_id, status, progress, created_at, started_at, completed_at, error_message
		FROM tasks WHERE task_type IN (%s) ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, placeholders)
	args := make([]interface{}, 0, len(filtered)+2)
	for _, taskType := range filtered {
		args = append(args, taskType)
	}
	args = append(args, limit, offset)
	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks by type: %w", err)
	}
	defer rows.Close()
	return scanTaskRows(rows)
}

type taskScanner interface {
	Scan(dest ...interface{}) error
}

func scanTask(scanner taskScanner, task *models.Task) error {
	var instanceID sql.NullString
	var createdAt, startedAt, completedAt sql.NullTime
	if err := scanner.Scan(&task.ID, &task.TaskType, &instanceID, &task.Status, &task.Progress,
		&createdAt, &startedAt, &completedAt, &task.ErrorMessage); err != nil {
		return err
	}
	if instanceID.Valid {
		task.InstanceID = instanceID.String
	}
	if createdAt.Valid {
		task.CreatedAt = createdAt.Time
	}
	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.Time
	}
	return nil
}

func scanTaskRows(rows *sql.Rows) ([]models.Task, error) {
	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		if err := scanTask(rows, &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	return tasks, nil
}
