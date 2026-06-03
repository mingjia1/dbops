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
		return r.createInMemory(dep)
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
		return r.getByIDInMemory(id)
	}
	return r.getByIDInDB(ctx, id)
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
		r.mu.Lock()
		defer r.mu.Unlock()
		if dep, ok := r.memDB[id]; ok {
			dep.Status = status
			dep.UpdatedAt = time.Now()
		}
		return nil
	}
	_, err := r.db.Pool.ExecContext(ctx, `UPDATE cluster_deployments SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}
