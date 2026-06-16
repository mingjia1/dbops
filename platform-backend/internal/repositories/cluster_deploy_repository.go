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

type ClusterDeployRepository struct {
	db *Database

	mu    sync.RWMutex
	memDB map[string]*models.ClusterDeployment
}

func NewClusterDeployRepository(db *Database) *ClusterDeployRepository {
	return &ClusterDeployRepository{
		db:    db,
		memDB: make(map[string]*models.ClusterDeployment),
	}
}

func (r *ClusterDeployRepository) Create(ctx context.Context, dep *models.ClusterDeployment) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	return r.createInDB(ctx, dep)
}

func (r *ClusterDeployRepository) createInMemory(dep *models.ClusterDeployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dep.ID == "" {
		dep.ID = uuid.New().String()
	}
	if dep.Status == "" {
		dep.Status = "pending"
	}
	now := time.Now()
	if dep.StartedAt != nil {
		startCopy := *dep.StartedAt
		dep.StartedAt = &startCopy
	}
	if dep.FinishedAt != nil {
		finishCopy := *dep.FinishedAt
		dep.FinishedAt = &finishCopy
	}
	dep.CreatedAt = now
	dep.UpdatedAt = now
	r.memDB[dep.ID] = dep
	return nil
}

func (r *ClusterDeployRepository) createInDB(ctx context.Context, dep *models.ClusterDeployment) error {
	if dep.ID == "" {
		dep.ID = uuid.New().String()
	}
	now := time.Now()
	dep.CreatedAt = now
	dep.UpdatedAt = now

	query := `INSERT INTO cluster_deployments (id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, dep.ID, dep.ClusterType, dep.Name, dep.Status, dep.RequestJSON, dep.PlanJSON, dep.CustomJSON, dep.StartedAt, dep.FinishedAt, dep.ErrorMessage, dep.CreatedAt, dep.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	return nil
}

func (r *ClusterDeployRepository) GetByID(ctx context.Context, id string) (*models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	return r.getByIDInDB(ctx, id)
}

func (r *ClusterDeployRepository) GetByName(ctx context.Context, name string) (*models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	return r.getByNameInDB(ctx, name)
}

func (r *ClusterDeployRepository) List(ctx context.Context, limit, offset int) ([]models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, created_at, updated_at FROM cluster_deployments ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	defer rows.Close()

	deployments := make([]models.ClusterDeployment, 0)
	for rows.Next() {
		var dep models.ClusterDeployment
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status,
			&dep.RequestJSON, &dep.PlanJSON, &dep.CustomJSON,
			&startedAt, &finishedAt, &dep.ErrorMessage,
			&dep.CreatedAt, &dep.UpdatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			dep.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			dep.FinishedAt = &finishedAt.Time
		}
		deployments = append(deployments, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deployments, nil
}

func (r *ClusterDeployRepository) getByIDInMemory(id string) (*models.ClusterDeployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dep, ok := r.memDB[id]
	if !ok {
		return nil, fmt.Errorf("deployment not found")
	}
	copy := *dep
	return &copy, nil
}

func (r *ClusterDeployRepository) scanClusterDeployment(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.ClusterDeployment, error) {
	var dep models.ClusterDeployment
	var startedAt, finishedAt sql.NullTime
	err := scanner.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status,
		&dep.RequestJSON, &dep.PlanJSON, &dep.CustomJSON,
		&startedAt, &finishedAt, &dep.ErrorMessage,
		&dep.CreatedAt, &dep.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deployment not found")
		}
		return nil, fmt.Errorf("failed to scan deployment: %w", err)
	}
	if startedAt.Valid {
		dep.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		dep.FinishedAt = &finishedAt.Time
	}
	return &dep, nil
}

func (r *ClusterDeployRepository) getByIDInDB(ctx context.Context, id string) (*models.ClusterDeployment, error) {
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, created_at, updated_at FROM cluster_deployments WHERE id = ?`
	return r.scanClusterDeployment(r.db.Pool.QueryRowContext(ctx, query, id))
}

func (r *ClusterDeployRepository) getByNameInDB(ctx context.Context, name string) (*models.ClusterDeployment, error) {
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, created_at, updated_at FROM cluster_deployments WHERE name = ? ORDER BY created_at DESC LIMIT 1`
	return r.scanClusterDeployment(r.db.Pool.QueryRowContext(ctx, query, name))
}

func (r *ClusterDeployRepository) UpdateStatus(ctx context.Context, id, status string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	query := `UPDATE cluster_deployments SET status = ?, updated_at = ?`
	args := []interface{}{status, now}
	if status == "running" || status == "in_progress" {
		query += `, started_at = COALESCE(started_at, ?)`
		args = append(args, now)
	}
	if isFinalStatus(status) {
		query += `, finished_at = ?`
		args = append(args, now)
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	_, err := r.db.Pool.ExecContext(ctx, query, args...)
	return err
}

func (r *ClusterDeployRepository) UpdateStatusWithError(ctx context.Context, id, status, errorMessage string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE cluster_deployments SET status = ?, error_message = ?, finished_at = ?, updated_at = ? WHERE id = ?`,
		status, errorMessage, now, now, id)
	return err
}

func (r *ClusterDeployRepository) UpdatePlan(ctx context.Context, id, planJSON string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `UPDATE cluster_deployments SET plan_json = ?, updated_at = ? WHERE id = ?`, planJSON, time.Now(), id)
	return err
}

func (r *ClusterDeployRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM cluster_deployments WHERE id = ?`, id)
	return err
}

func isFinalStatus(status string) bool {
	switch status {
	case "completed", "success", "failed", "destroyed", "cancelled":
		return true
	default:
		return false
	}
}
