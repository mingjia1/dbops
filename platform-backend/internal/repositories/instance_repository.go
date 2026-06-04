package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type InstanceRepository struct {
	db *Database

	mu    sync.RWMutex
	memDB map[string]*models.Instance
	conns map[string]models.InstanceConnection
	vers  map[string]models.InstanceVersion
}

func NewInstanceRepository(db *Database) *InstanceRepository {
	return &InstanceRepository{
		db:    db,
		memDB: make(map[string]*models.Instance),
		conns: make(map[string]models.InstanceConnection),
		vers:  make(map[string]models.InstanceVersion),
	}
}

func (r *InstanceRepository) Create(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return r.createInMemory(instance)
	}
	return r.createInDB(ctx, instance)
}

func (r *InstanceRepository) createInMemory(instance *models.Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = time.Now()
	}
	instance.UpdatedAt = time.Now()

	for _, existing := range r.memDB {
		if existing.Name == instance.Name {
			return fmt.Errorf("instance with name %s already exists", instance.Name)
		}
	}

	r.memDB[instance.ID] = instance

	if instance.Connection.InstanceID != "" {
		if instance.Connection.ID == "" {
			instance.Connection.ID = uuid.New().String()
		}
		instance.Connection.InstanceID = instance.ID
		r.conns[instance.Connection.ID] = instance.Connection
	}

	if instance.Version.InstanceID != "" {
		if instance.Version.ID == "" {
			instance.Version.ID = uuid.New().String()
		}
		instance.Version.InstanceID = instance.ID
		r.vers[instance.Version.ID] = instance.Version
	}

	return nil
}

func (r *InstanceRepository) createInDB(ctx context.Context, instance *models.Instance) error {
	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = time.Now()
	}
	instance.UpdatedAt = time.Now()

	query := `
		INSERT INTO instances (id, name, cluster_id, host_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		instance.ID, instance.Name, instance.ClusterID, nullableStringPtr(instance.HostID), instance.CreatedAt, instance.UpdatedAt)

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
		r.mu.Lock()
		defer r.mu.Unlock()
		if conn.ID == "" {
			conn.ID = uuid.New().String()
		}
		r.conns[conn.ID] = *conn
		return nil
	}

	if conn.ID == "" {
		conn.ID = uuid.New().String()
	}
	query := `
		INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		conn.ID, conn.InstanceID, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, conn.SSLEnabled)
	return err
}

func (r *InstanceRepository) GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	if r.db == nil || r.db.Pool == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, conn := range r.conns {
			if conn.InstanceID == instanceID {
				return &conn, nil
			}
		}
		return nil, fmt.Errorf("connection not found for instance %s", instanceID)
	}

	query := `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled
		FROM instance_connections WHERE instance_id = ?
	`
	conn := &models.InstanceConnection{}
	err := r.db.Pool.QueryRowContext(ctx, query, instanceID).Scan(
		&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("connection not found for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	return conn, nil
}

func (r *InstanceRepository) CreateVersion(ctx context.Context, version *models.InstanceVersion) error {
	if r.db == nil || r.db.Pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		if version.ID == "" {
			version.ID = uuid.New().String()
		}
		r.vers[version.ID] = *version
		return nil
	}

	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	query := `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		version.ID, version.InstanceID, version.Flavor, version.Version, version.FullVersion,
		version.ReleaseDate, version.EOLDate, version.IsLTS, version.Features, version.Engines)
	return err
}

func (r *InstanceRepository) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return r.getByIDInMemory(id)
	}
	return r.getByIDInDB(ctx, id)
}

func (r *InstanceRepository) getByIDInMemory(id string) (*models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, ok := r.memDB[id]
	if !ok {
		return nil, fmt.Errorf("instance not found")
	}
	copy := *inst
	return &copy, nil
}

func (r *InstanceRepository) getByIDInDB(ctx context.Context, id string) (*models.Instance, error) {
	query := `
		SELECT id, name, cluster_id, host_id, created_at, updated_at
		FROM instances WHERE id = ?
	`

	instance := &models.Instance{}
	var hostID sql.NullString
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&instance.ID, &instance.Name, &instance.ClusterID, &hostID, &instance.CreatedAt, &instance.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
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
		return r.listInMemory(limit, offset)
	}
	return r.listInDB(ctx, limit, offset)
}

func (r *InstanceRepository) listInMemory(limit, offset int) ([]models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]models.Instance, 0, len(r.memDB))
	for _, inst := range r.memDB {
		all = append(all, *inst)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if offset >= len(all) {
		return []models.Instance{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *InstanceRepository) listInDB(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	query := `
		SELECT id, name, cluster_id, host_id, created_at, updated_at
		FROM instances ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()

	instances := make([]models.Instance, 0)
	for rows.Next() {
		var instance models.Instance
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

	return instances, nil
}

func (r *InstanceRepository) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return r.listByHostIDInMemory(hostID, limit, offset)
	}
	return r.listByHostIDInDB(ctx, hostID, limit, offset)
}

func (r *InstanceRepository) ListByClusterID(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return r.listByClusterIDInMemory(clusterID)
	}
	return r.listByClusterIDInDB(ctx, clusterID)
}

func (r *InstanceRepository) listByClusterIDInMemory(clusterID string) ([]*models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*models.Instance, 0)
	for _, inst := range r.memDB {
		if inst != nil && inst.ClusterID == clusterID {
			copy := *inst
			out = append(out, &copy)
		}
	}
	return out, nil
}

func (r *InstanceRepository) listByClusterIDInDB(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	query := `
		SELECT id, name, cluster_id, host_id, created_at, updated_at
		FROM instances WHERE cluster_id = ? ORDER BY created_at ASC
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, clusterID)
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
	return instances, nil
}

func (r *InstanceRepository) listByHostIDInMemory(hostID string, limit, offset int) ([]models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]models.Instance, 0)
	for _, inst := range r.memDB {
		if inst.HostID != nil && *inst.HostID == hostID {
			all = append(all, *inst)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if offset >= len(all) {
		return []models.Instance{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *InstanceRepository) listByHostIDInDB(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	query := `
		SELECT id, name, cluster_id, host_id, created_at, updated_at
		FROM instances WHERE host_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, hostID, limit, offset)
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

	return instances, nil
}

func (r *InstanceRepository) Update(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return r.updateInMemory(instance)
	}
	return r.updateInDB(ctx, instance)
}

func (r *InstanceRepository) updateInMemory(instance *models.Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.memDB[instance.ID]
	if !ok {
		return fmt.Errorf("instance not found")
	}
	if instance.Name != "" {
		existing.Name = instance.Name
	}
	if instance.ClusterID != "" {
		existing.ClusterID = instance.ClusterID
	}
	existing.HostID = instance.HostID
	existing.UpdatedAt = time.Now()
	return nil
}

func (r *InstanceRepository) updateInDB(ctx context.Context, instance *models.Instance) error {
	instance.UpdatedAt = time.Now()
	query := `
		UPDATE instances SET name = ?, cluster_id = ?, host_id = ?, updated_at = ? WHERE id = ?
	`
	_, err := r.db.Pool.ExecContext(ctx, query, instance.Name, instance.ClusterID, nullableStringPtr(instance.HostID), instance.UpdatedAt, instance.ID)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}
	return nil
}

func (r *InstanceRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.memDB, id)
		return nil
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
