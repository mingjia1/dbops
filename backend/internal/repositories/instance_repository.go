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

	tx, err := r.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
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
		_, err = tx.ExecContext(ctx, `
			INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled, basedir, datadir, os_user, package_url, version_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			instance.Connection.ID, instance.Connection.InstanceID, instance.Connection.Host,
			instance.Connection.Port, instance.Connection.Username, instance.Connection.PasswordEncrypted,
			instance.Connection.SSLEnabled, instance.Connection.Basedir, instance.Connection.Datadir,
			instance.Connection.OSUser, instance.Connection.PackageURL, instance.Connection.VersionID,
		)
		if err != nil {
			return fmt.Errorf("failed to create instance connection: %w", err)
		}
	}
	if instance.Version.InstanceID != "" {
		instance.Version.ID = uuid.New().String()
		instance.Version.InstanceID = instance.ID
		_, err = tx.ExecContext(ctx, `DELETE FROM instance_versions WHERE instance_id = ?`, instance.Version.InstanceID)
		if err != nil {
			return fmt.Errorf("failed to replace existing version: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			instance.Version.ID, instance.Version.InstanceID, instance.Version.Flavor,
			instance.Version.Version, instance.Version.FullVersion,
			nullableTime(&instance.Version.ReleaseDate), nullableTime(&instance.Version.EOLDate),
			instance.Version.IsLTS, instance.Version.Features, instance.Version.Engines,
		)
		if err != nil {
			return fmt.Errorf("failed to create instance version: %w", err)
		}
	}

	return tx.Commit()
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

	tx, err := r.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM instance_versions WHERE instance_id = ?`, version.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to replace existing version: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ID, version.InstanceID, version.Flavor, version.Version, version.FullVersion,
		nullableTime(&version.ReleaseDate), nullableTime(&version.EOLDate), version.IsLTS, version.Features, version.Engines,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateVersion updates an existing version record for an instance.
func (r *InstanceRepository) UpdateVersion(ctx context.Context, version *models.InstanceVersion) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if version.ID == "" {
		return fmt.Errorf("version ID is required for update")
	}
	res, err := r.db.Pool.ExecContext(ctx, `
		UPDATE instance_versions
		SET flavor = ?, version = ?, full_version = ?, release_date = ?, eol_date = ?, is_lts = ?, features = ?, engines = ?
		WHERE id = ? AND instance_id = ?`,
		version.Flavor, version.Version, version.FullVersion,
		nullableTime(&version.ReleaseDate), nullableTime(&version.EOLDate),
		version.IsLTS, version.Features, version.Engines,
		version.ID, version.InstanceID,
	)
	if err != nil {
		return fmt.Errorf("failed to update version: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if n == 0 {
		// No row updated — insert as new record
		return r.CreateVersion(ctx, version)
	}
	return nil
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
	id := uuid.New().String()
	if r.db.IsSQLite() {
		_, err := r.db.Pool.ExecContext(ctx, `
			INSERT OR REPLACE INTO instance_topologies (id, instance_id, cluster_id, master_id, slave_ids, replication_mode)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, instanceID, nullableString(topology.ClusterID), nullableString(topology.MasterID),
			nullableString(topology.SlaveIDs), nullableString(topology.ReplicationMode))
		if err != nil {
			return fmt.Errorf("failed to upsert topology: %w", err)
		}
	} else {
		_, err := r.db.Pool.ExecContext(ctx, `
			INSERT INTO instance_topologies (id, instance_id, cluster_id, master_id, slave_ids, replication_mode)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE cluster_id=VALUES(cluster_id), master_id=VALUES(master_id),
				slave_ids=VALUES(slave_ids), replication_mode=VALUES(replication_mode)`,
			id, instanceID, nullableString(topology.ClusterID), nullableString(topology.MasterID),
			nullableString(topology.SlaveIDs), nullableString(topology.ReplicationMode))
		if err != nil {
			return fmt.Errorf("failed to upsert topology: %w", err)
		}
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
	id := uuid.New().String()
	if r.db.IsSQLite() {
		_, err := r.db.Pool.ExecContext(ctx, `
			INSERT OR REPLACE INTO instance_statuses (id, instance_id, run_status, health_status, role, replication_status, seconds_behind_master, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, instanceID, status.RunStatus, status.HealthStatus, status.Role,
			nullableString(status.ReplicationStatus), status.SecondsBehindMaster, now)
		if err != nil {
			return fmt.Errorf("failed to upsert status: %w", err)
		}
	} else {
		_, err := r.db.Pool.ExecContext(ctx, `
			INSERT INTO instance_statuses (id, instance_id, run_status, health_status, role, replication_status, seconds_behind_master, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE run_status=VALUES(run_status), health_status=VALUES(health_status),
				role=VALUES(role), replication_status=VALUES(replication_status),
				seconds_behind_master=VALUES(seconds_behind_master), updated_at=VALUES(updated_at)`,
			id, instanceID, status.RunStatus, status.HealthStatus, status.Role,
			nullableString(status.ReplicationStatus), status.SecondsBehindMaster, now)
		if err != nil {
			return fmt.Errorf("failed to upsert status: %w", err)
		}
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

// BatchGetStatuses fetches instance_statuses for a set of instance IDs in one query.
func (r *InstanceRepository) BatchGetStatuses(ctx context.Context, ids []string) (map[string]*models.InstanceStatus, error) {
	if r.db == nil || r.db.Pool == nil || len(ids) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query, args, err := sqlxIn(`SELECT id, instance_id, run_status, health_status, role, COALESCE(replication_status,''), COALESCE(seconds_behind_master,-1), updated_at FROM instance_statuses WHERE instance_id IN (?)`, args)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get statuses: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*models.InstanceStatus, len(ids))
	for rows.Next() {
		var s models.InstanceStatus
		var id, instID string
		var lag sql.NullInt64
		if err := rows.Scan(&id, &instID, &s.RunStatus, &s.HealthStatus, &s.Role, &s.ReplicationStatus, &lag, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.InstanceID = instID
		if lag.Valid {
			s.SecondsBehindMaster = int(lag.Int64)
		}
		result[instID] = &s
	}
	return result, rows.Err()
}

// BatchGetTopologies fetches instance_topologies for a set of instance IDs in one query.
func (r *InstanceRepository) BatchGetTopologies(ctx context.Context, ids []string) (map[string]*models.InstanceTopology, error) {
	if r.db == nil || r.db.Pool == nil || len(ids) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query, args, err := sqlxIn(`SELECT id, instance_id, COALESCE(cluster_id,''), COALESCE(master_id,''), COALESCE(slave_ids,''), COALESCE(replication_mode,'') FROM instance_topologies WHERE instance_id IN (?)`, args)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get topologies: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*models.InstanceTopology, len(ids))
	for rows.Next() {
		var t models.InstanceTopology
		var id, instID string
		if err := rows.Scan(&id, &instID, &t.ClusterID, &t.MasterID, &t.SlaveIDs, &t.ReplicationMode); err != nil {
			return nil, err
		}
		t.InstanceID = instID
		result[instID] = &t
	}
	return result, rows.Err()
}

// BatchGetConnections fetches instance_connections for a set of instance IDs in one query.
func (r *InstanceRepository) BatchGetConnections(ctx context.Context, ids []string) (map[string]*models.InstanceConnection, error) {
	if r.db == nil || r.db.Pool == nil || len(ids) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query, args, err := sqlxIn(`SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled, COALESCE(basedir,''), COALESCE(datadir,''), COALESCE(os_user,''), COALESCE(package_url,''), COALESCE(version_id,'') FROM instance_connections WHERE instance_id IN (?)`, args)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get connections: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*models.InstanceConnection, len(ids))
	for rows.Next() {
		var conn models.InstanceConnection
		if err := rows.Scan(
			&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled,
			&conn.Basedir, &conn.Datadir, &conn.OSUser, &conn.PackageURL, &conn.VersionID,
		); err != nil {
			return nil, err
		}
		result[conn.InstanceID] = &conn
	}
	return result, rows.Err()
}

// BatchGetVersions fetches instance_versions for a set of instance IDs in one query.
func (r *InstanceRepository) BatchGetVersions(ctx context.Context, ids []string) (map[string]*models.InstanceVersion, error) {
	if r.db == nil || r.db.Pool == nil || len(ids) == 0 {
		return nil, nil
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query, args, err := sqlxIn(`SELECT id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, COALESCE(features,''), COALESCE(engines,'') FROM instance_versions WHERE instance_id IN (?)`, args)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get versions: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*models.InstanceVersion, len(ids))
	for rows.Next() {
		var v models.InstanceVersion
		var releaseDate, eolDate sql.NullTime
		if err := rows.Scan(&v.ID, &v.InstanceID, &v.Flavor, &v.Version, &v.FullVersion, &releaseDate, &eolDate, &v.IsLTS, &v.Features, &v.Engines); err != nil {
			return nil, err
		}
		if releaseDate.Valid {
			v.ReleaseDate = releaseDate.Time
		}
		if eolDate.Valid {
			v.EOLDate = eolDate.Time
		}
		result[v.InstanceID] = &v
	}
	return result, rows.Err()
}

// EnrichInstancesBatch populates status/topology/connection/version for a slice of instances in 4 batch queries.
func (r *InstanceRepository) EnrichInstancesBatch(ctx context.Context, instances []models.Instance) error {
	if len(instances) == 0 {
		return nil
	}
	ids := make([]string, len(instances))
	for i := range instances {
		ids[i] = instances[i].ID
	}
	statuses, err := r.BatchGetStatuses(ctx, ids)
	if err != nil {
		return err
	}
	topologies, err := r.BatchGetTopologies(ctx, ids)
	if err != nil {
		return err
	}
	connections, err := r.BatchGetConnections(ctx, ids)
	if err != nil {
		return err
	}
	versions, err := r.BatchGetVersions(ctx, ids)
	if err != nil {
		return err
	}
	for i := range instances {
		id := instances[i].ID
		if s, ok := statuses[id]; ok {
			instances[i].Status = *s
		}
		if t, ok := topologies[id]; ok {
			instances[i].Topology = *t
		}
		if c, ok := connections[id]; ok {
			instances[i].Connection = *c
			instances[i].Connection.PasswordEncrypted = ""
		}
		if v, ok := versions[id]; ok {
			instances[i].Version = *v
		}
	}
	return nil
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

// sqlxIn builds a "col IN (?,?,?)" clause for any number of args (replaces sqlx.In).
func sqlxIn(query string, args []interface{}) (string, []interface{}, error) {
	n := len(args)
	if n == 0 {
		return "", nil, fmt.Errorf("sqlxIn: empty args")
	}
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	idx := len(query)
	for i, c := range query {
		if c == '?' {
			idx = i
			break
		}
	}
	newQuery := query[:idx] + strings.Join(placeholders, ",") + query[idx+1:]
	return newQuery, args, nil
}
