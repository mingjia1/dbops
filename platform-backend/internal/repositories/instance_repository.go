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

type InstanceRepository struct {
	db *Database
}

func NewInstanceRepository(db *Database) *InstanceRepository {
	return &InstanceRepository{db: db}
}

// AttachStore 保留兼容旧调用 - SQLite 本身就是持久化层, 无需 JSON 兜底.
func (r *InstanceRepository) AttachStore(store *JSONStore) { _ = store }

func (r *InstanceRepository) Create(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = time.Now().UTC()
	}
	instance.UpdatedAt = time.Now().UTC()

	var existing string
	err := r.db.Pool.QueryRowContext(ctx, `SELECT id FROM instances WHERE name = ? LIMIT 1`, instance.Name).Scan(&existing)
	if err == nil {
		return fmt.Errorf("instance with name %s already exists", instance.Name)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check instance name: %w", err)
	}

	_, err = r.db.Pool.ExecContext(ctx,
		`INSERT INTO instances (id, name, cluster_id, host_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		instance.ID, instance.Name, nullableString(instance.ClusterID), nullableStringPtr(instance.HostID),
		instance.CreatedAt, instance.UpdatedAt,
	)
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
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if conn.ID == "" {
		conn.ID = uuid.New().String()
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.InstanceID, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, conn.SSLEnabled,
	)
	return err
}

func (r *InstanceRepository) GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled
		FROM instance_connections WHERE instance_id = ?`, instanceID)
	conn := &models.InstanceConnection{}
	if err := row.Scan(&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("connection not found for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	return conn, nil
}

func (r *InstanceRepository) CreateVersion(ctx context.Context, version *models.InstanceVersion) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.InstanceID, version.Flavor, version.Version, version.FullVersion,
		nullableTime(&version.ReleaseDate), nullableTime(&version.EOLDate), version.IsLTS, version.Features, version.Engines,
	)
	return err
}

func (r *InstanceRepository) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, name, cluster_id, host_id, created_at, updated_at FROM instances WHERE id = ?`, id)
	instance := &models.Instance{}
	var hostID sql.NullString
	if err := row.Scan(&instance.ID, &instance.Name, &instance.ClusterID, &hostID, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("instance not found")
		}
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if hostID.Valid {
		s := hostID.String
		instance.HostID = &s
	}
	return instance, nil
}

func (r *InstanceRepository) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, name, cluster_id, host_id, created_at, updated_at FROM instances ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()

	instances := make([]models.Instance, 0)
	for rows.Next() {
		var instance models.Instance
		var clusterID, hostID sql.NullString
		if err := rows.Scan(&instance.ID, &instance.Name, &clusterID, &hostID, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
			return nil, err
		}
		if clusterID.Valid {
			instance.ClusterID = clusterID.String
		}
		if hostID.Valid {
			s := hostID.String
			instance.HostID = &s
		}
		instances = append(instances, instance)
	}
	return instances, rows.Err()
}

func (r *InstanceRepository) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, name, cluster_id, host_id, created_at, updated_at FROM instances WHERE host_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		hostID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances by host: %w", err)
	}
	defer rows.Close()

	instances := make([]models.Instance, 0)
	for rows.Next() {
		var instance models.Instance
		var hid sql.NullString
		if err := rows.Scan(&instance.ID, &instance.Name, &instance.ClusterID, &hid, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
			return nil, err
		}
		if hid.Valid {
			s := hid.String
			instance.HostID = &s
		}
		instances = append(instances, instance)
	}
	return instances, rows.Err()
}

func (r *InstanceRepository) ListByClusterID(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, name, cluster_id, host_id, created_at, updated_at FROM instances WHERE cluster_id = ? ORDER BY created_at ASC`,
		clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances by cluster: %w", err)
	}
	defer rows.Close()

	instances := make([]*models.Instance, 0)
	for rows.Next() {
		instance := &models.Instance{}
		var hostID sql.NullString
		if err := rows.Scan(&instance.ID, &instance.Name, &instance.ClusterID, &hostID, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
			return nil, err
		}
		if hostID.Valid {
			s := hostID.String
			instance.HostID = &s
		}
		instances = append(instances, instance)
	}
	return instances, rows.Err()
}

func (r *InstanceRepository) Update(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	instance.UpdatedAt = time.Now().UTC()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE instances SET name = ?, cluster_id = ?, host_id = ?, updated_at = ? WHERE id = ?`,
		instance.Name, instance.ClusterID, nullableStringPtr(instance.HostID), instance.UpdatedAt, instance.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}
	return nil
}

func (r *InstanceRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}
	return nil
}

func nullableStringPtr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}
