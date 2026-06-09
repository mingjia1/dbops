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

	query := `INSERT INTO cluster_deployments (id, cluster_type, name, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, dep.ID, dep.ClusterType, dep.Name, dep.Status, dep.CreatedAt, dep.UpdatedAt)
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

func (r *ClusterDeployRepository) List(ctx context.Context, limit, offset int) ([]models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, cluster_type, name, status, created_at, updated_at FROM cluster_deployments ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	defer rows.Close()

	deployments := make([]models.ClusterDeployment, 0)
	for rows.Next() {
		var dep models.ClusterDeployment
		if err := rows.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status, &dep.CreatedAt, &dep.UpdatedAt); err != nil {
			return nil, err
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

func (r *ClusterDeployRepository) getByIDInDB(ctx context.Context, id string) (*models.ClusterDeployment, error) {
	query := `SELECT id, cluster_type, name, status, created_at, updated_at FROM cluster_deployments WHERE id = ?`
	dep := &models.ClusterDeployment{}
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status, &dep.CreatedAt, &dep.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deployment not found")
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	return dep, nil
}

func (r *ClusterDeployRepository) UpdateStatus(ctx context.Context, id, status string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `UPDATE cluster_deployments SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

func (r *ClusterDeployRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM cluster_deployments WHERE id = ?`, id)
	return err
}
