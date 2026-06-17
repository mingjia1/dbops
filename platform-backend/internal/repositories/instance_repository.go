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
		INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled, basedir, datadir, os_user, package_url, version_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.InstanceID, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, conn.SSLEnabled,
		conn.Basedir, conn.Datadir, conn.OSUser, conn.PackageURL, conn.VersionID,
	)
	return err
}

func (r *InstanceRepository) GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled,
			COALESCE(basedir,''), COALESCE(datadir,''), COALESCE(os_user,''), COALESCE(package_url,''), COALESCE(version_id,'')
		FROM instance_connections WHERE instance_id = ?`, instanceID)
	conn := &models.InstanceConnection{}
	if err := row.Scan(
		&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled,
		&conn.Basedir, &conn.Datadir, &conn.OSUser, &conn.PackageURL, &conn.VersionID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("connection not found for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	return conn, nil
}

func (r *InstanceRepository) UpdateConnectionPassword(ctx context.Context, instanceID, passwordEncrypted string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	res, err := r.db.Pool.ExecContext(ctx,
		`UPDATE instance_connections SET password_encrypted = ? WHERE instance_id = ?`,
		passwordEncrypted, instanceID,
	)
	if err != nil {
		return fmt.Errorf("failed to update connection password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("connection not found for instance %s", instanceID)
	}
	return nil
}

func (r *InstanceRepository) UpdateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	res, err := r.db.Pool.ExecContext(ctx, `
		UPDATE instance_connections
		SET host = ?, port = ?, username = ?, password_encrypted = ?, ssl_enabled = ?,
			basedir = ?, datadir = ?, os_user = ?, package_url = ?, version_id = ?
		WHERE instance_id = ?`,
		conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, conn.SSLEnabled,
		conn.Basedir, conn.Datadir, conn.OSUser, conn.PackageURL, conn.VersionID, conn.InstanceID,
	)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("connection not found for instance %s", conn.InstanceID)
	}
	return nil
}

func (r *InstanceRepository) CreateVersion(ctx context.Context, version *models.InstanceVersion) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM instance_versions WHERE instance_id = ?`, version.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to replace existing version: %w", err)
	}
	_, err = r.db.Pool.ExecContext(ctx, `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.InstanceID, version.Flavor, version.Version, version.FullVersion,
		nullableTime(&version.ReleaseDate), nullableTime(&version.EOLDate), version.IsLTS, version.Features, version.Engines,
	)
	return err
}

func (r *InstanceRepository) GetVersion(ctx context.Context, instanceID string) (*models.InstanceVersion, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, COALESCE(features,''), COALESCE(engines,'')
		FROM instance_versions WHERE instance_id = ?`, instanceID)
	var v models.InstanceVersion
	var releaseDate, eolDate sql.NullTime
	if err := row.Scan(&v.ID, &v.InstanceID, &v.Flavor, &v.Version, &v.FullVersion, &releaseDate, &eolDate, &v.IsLTS, &v.Features, &v.Engines); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("version not detected for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	if releaseDate.Valid {
		v.ReleaseDate = releaseDate.Time
	}
	if eolDate.Valid {
		v.EOLDate = eolDate.Time
	}
	return &v, nil
}

func (r *InstanceRepository) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, name, cluster_id, host_id, created_at, updated_at FROM instances WHERE id = ?`, id)
	instance := &models.Instance{}
	var clusterID, hostID sql.NullString
	if err := row.Scan(&instance.ID, &instance.Name, &clusterID, &hostID, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("instance not found")
		}
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if clusterID.Valid {
		instance.ClusterID = clusterID.String
	}
	if hostID.Valid {
		s := hostID.String
		instance.HostID = &s
	}
	// 加载 instance_statuses.role / health_status 等运行时字段, 否则 SwitchService 等调用方
	// 永远读到 Role="" 就会把所有实例误判为 replica 角色.
	if status, err := r.GetStatus(ctx, id); err == nil {
		instance.Status = *status
	}
	// 加载 instance_topologies (master_id 等), 否则依赖 topology 推导的角色关系会丢失.
	if topology, err := r.GetTopology(ctx, id); err == nil {
		instance.Topology = *topology
	}
	if conn, err := r.GetConnection(ctx, id); err == nil {
		instance.Connection = *conn
		instance.Connection.PasswordEncrypted = ""
	}
	if version, err := r.GetVersion(ctx, id); err == nil {
		instance.Version = *version
	}
	return instance, nil
}

// GetTopology 读 instance_topologies 表, 返回该实例的拓扑信息.
func (r *InstanceRepository) GetTopology(ctx context.Context, instanceID string) (*models.InstanceTopology, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, COALESCE(cluster_id,''), COALESCE(master_id,''), COALESCE(slave_ids,''), COALESCE(replication_mode,'')
		FROM instance_topologies WHERE instance_id = ?`, instanceID)
	var t models.InstanceTopology
	var id, instID string
	if err := row.Scan(&id, &instID, &t.ClusterID, &t.MasterID, &t.SlaveIDs, &t.ReplicationMode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("topology not found for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get topology: %w", err)
	}
	t.InstanceID = instID
	return &t, nil
}

// UpsertTopology 写 instance_topologies 行, 存在则 update 否则 insert.
func (r *InstanceRepository) UpsertTopology(ctx context.Context, instanceID string, topology *models.InstanceTopology) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	var existing string
	err := r.db.Pool.QueryRowContext(ctx, `SELECT id FROM instance_topologies WHERE instance_id = ?`, instanceID).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = r.db.Pool.ExecContext(ctx, `
			INSERT INTO instance_topologies (id, instance_id, cluster_id, master_id, slave_ids, replication_mode)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), instanceID, nullableString(topology.ClusterID), nullableString(topology.MasterID), nullableString(topology.SlaveIDs), nullableString(topology.ReplicationMode))
	} else if err == nil {
		_, err = r.db.Pool.ExecContext(ctx, `
			UPDATE instance_topologies SET cluster_id=?, master_id=?, slave_ids=?, replication_mode=? WHERE instance_id=?`,
			nullableString(topology.ClusterID), nullableString(topology.MasterID), nullableString(topology.SlaveIDs), nullableString(topology.ReplicationMode), instanceID)
	}
	if err != nil {
		return fmt.Errorf("failed to upsert topology: %w", err)
	}
	return nil
}

// GetStatus 读 instance_statuses 表, 返回该实例的运行时状态.
func (r *InstanceRepository) GetStatus(ctx context.Context, instanceID string) (*models.InstanceStatus, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, run_status, health_status, role, COALESCE(replication_status,''), COALESCE(seconds_behind_master,-1), updated_at
		FROM instance_statuses WHERE instance_id = ?`, instanceID)
	var s models.InstanceStatus
	var id, instID string
	var lag sql.NullInt64
	if err := row.Scan(&id, &instID, &s.RunStatus, &s.HealthStatus, &s.Role, &s.ReplicationStatus, &lag, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("status not found for instance %s", instanceID)
		}
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	s.InstanceID = instID
	if lag.Valid {
		s.SecondsBehindMaster = int(lag.Int64)
	}
	return &s, nil
}

// UpsertStatus 写 instance_statuses 行, 存在则 update 否则 insert.
func (r *InstanceRepository) UpsertStatus(ctx context.Context, instanceID string, status *models.InstanceStatus) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	now := time.Now().UTC()
	status.UpdatedAt = now
	// 先看是否存在
	var existing string
	err := r.db.Pool.QueryRowContext(ctx, `SELECT id FROM instance_statuses WHERE instance_id = ?`, instanceID).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = r.db.Pool.ExecContext(ctx, `
			INSERT INTO instance_statuses (id, instance_id, run_status, health_status, role, replication_status, seconds_behind_master, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), instanceID, status.RunStatus, status.HealthStatus, status.Role,
			nullableString(status.ReplicationStatus), status.SecondsBehindMaster, now)
	} else if err == nil {
		_, err = r.db.Pool.ExecContext(ctx, `
			UPDATE instance_statuses SET run_status=?, health_status=?, role=?, replication_status=?, seconds_behind_master=?, updated_at=? WHERE instance_id=?`,
			status.RunStatus, status.HealthStatus, status.Role, nullableString(status.ReplicationStatus), status.SecondsBehindMaster, now, instanceID)
	}
	if err != nil {
		return fmt.Errorf("failed to upsert status: %w", err)
	}
	return nil
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

func (r *InstanceRepository) Update(ctx context.Context, instance *models.Instance) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	instance.UpdatedAt = time.Now().UTC()
	res, err := r.db.Pool.ExecContext(ctx,
		`UPDATE instances SET name = ?, cluster_id = ?, host_id = ?, updated_at = ? WHERE id = ?`,
		instance.Name, instance.ClusterID, nullableStringPtr(instance.HostID), instance.UpdatedAt, instance.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}
	// P0-5: 不能默默吞掉 0 rows affected; 0 表示 instance 不存在, 业务应感知.
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("instance not found: %s", instance.ID)
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
