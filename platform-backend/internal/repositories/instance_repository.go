package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type InstanceRepository struct {
	db *Database
}

func NewInstanceRepository(db *Database) *InstanceRepository {
	return &InstanceRepository{db: db}
}

func (r *InstanceRepository) Create(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	
	instance.ID = uuid.New().String()
	
	query := `
		INSERT INTO instances (id, name, cluster_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
		instance.ID, instance.Name, instance.ClusterID, instance.CreatedAt, instance.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}
	
	if instance.Connection.InstanceID != "" {
		instance.Connection.ID = uuid.New().String()
		instance.Connection.InstanceID = instance.ID
		if err := r.CreateConnection(ctx, &instance.Connection); err != nil {
			return err
		}
	}
	
	if instance.Version.InstanceID != "" {
		instance.Version.ID = uuid.New().String()
		instance.Version.InstanceID = instance.ID
		if err := r.CreateVersion(ctx, &instance.Version); err != nil {
			return err
		}
	}
	
	return nil
}

func (r *InstanceRepository) CreateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	if conn.ID == "" {
		conn.ID = uuid.New().String()
	}
	query := `
		INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		conn.ID, conn.InstanceID, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, conn.SSLEnabled)
	return err
}

func (r *InstanceRepository) CreateVersion(ctx context.Context, version *models.InstanceVersion) error {
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	query := `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		version.ID, version.InstanceID, version.Flavor, version.Version, version.FullVersion,
		version.ReleaseDate, version.EOLDate, version.IsLTS, version.Features, version.Engines)
	return err
}

func (r *InstanceRepository) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	query := `
		SELECT id, name, cluster_id, created_at, updated_at
		FROM instances WHERE id = $1
	`
	
	instance := &models.Instance{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&instance.ID, &instance.Name, &instance.ClusterID, &instance.CreatedAt, &instance.UpdatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("instance not found")
		}
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	
	return instance, nil
}

func (r *InstanceRepository) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.Instance{}, nil
	}
	
	query := `
		SELECT id, name, cluster_id, created_at, updated_at
		FROM instances ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()
	
	var instances []models.Instance
	for rows.Next() {
		var instance models.Instance
		if err := rows.Scan(&instance.ID, &instance.Name, &instance.ClusterID, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	
	return instances, nil
}

func (r *InstanceRepository) Update(ctx context.Context, instance *models.Instance) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	
	query := `
		UPDATE instances SET name = $2, cluster_id = $3, updated_at = $4 WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, instance.ID, instance.Name, instance.ClusterID, instance.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}
	return nil
}

func (r *InstanceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM instances WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}
	return nil
}