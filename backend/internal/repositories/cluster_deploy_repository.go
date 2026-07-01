package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type ClusterDeployRepository struct {
	db *Database
}

func NewClusterDeployRepository(db *Database) *ClusterDeployRepository {
	return &ClusterDeployRepository{
		db: db,
	}
}

func (r *ClusterDeployRepository) Create(ctx context.Context, dep *models.ClusterDeployment) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	return r.createInDB(ctx, dep)
}

func (r *ClusterDeployRepository) createInDB(ctx context.Context, dep *models.ClusterDeployment) error {
	if dep.ID == "" {
		dep.ID = uuid.New().String()
	}
	now := time.Now()
	dep.CreatedAt = now
	dep.UpdatedAt = now

	query := `INSERT INTO cluster_deployments (id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, cluster_id, display_name, arch, nodes, mysql_version, config_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, dep.ID, dep.ClusterType, dep.Name, dep.Status, dep.RequestJSON, dep.PlanJSON, dep.CustomJSON, dep.StartedAt, dep.FinishedAt, dep.ErrorMessage, dep.ClusterID, dep.DisplayName, dep.Arch, dep.Nodes, dep.MySQLVersion, dep.ConfigJSON, dep.CreatedAt, dep.UpdatedAt)
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
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, cluster_id, display_name, arch, nodes, mysql_version, config_json, created_at, updated_at FROM cluster_deployments ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	defer rows.Close()

	deployments := make([]models.ClusterDeployment, 0)
	for rows.Next() {
		var dep models.ClusterDeployment
		var startedAt, finishedAt sql.NullTime
		var clusterID, displayName, arch, mysqlVersion, configJSON sql.NullString
		var nodes sql.NullInt64
		if err := rows.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status,
			&dep.RequestJSON, &dep.PlanJSON, &dep.CustomJSON,
			&startedAt, &finishedAt, &dep.ErrorMessage,
			&clusterID, &displayName, &arch, &nodes, &mysqlVersion, &configJSON,
			&dep.CreatedAt, &dep.UpdatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			dep.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			dep.FinishedAt = &finishedAt.Time
		}
		if clusterID.Valid {
			dep.ClusterID = clusterID.String
		}
		if displayName.Valid {
			dep.DisplayName = displayName.String
		}
		if arch.Valid {
			dep.Arch = arch.String
		}
		if nodes.Valid {
			dep.Nodes = int(nodes.Int64)
		}
		if mysqlVersion.Valid {
			dep.MySQLVersion = mysqlVersion.String
		}
		if configJSON.Valid {
			dep.ConfigJSON = configJSON.String
		}
		deployments = append(deployments, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deployments, nil
}

func (r *ClusterDeployRepository) scanClusterDeployment(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.ClusterDeployment, error) {
	var dep models.ClusterDeployment
	var startedAt, finishedAt sql.NullTime
	var clusterID, displayName, arch, mysqlVersion, configJSON sql.NullString
	var nodes sql.NullInt64
	err := scanner.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status,
		&dep.RequestJSON, &dep.PlanJSON, &dep.CustomJSON,
		&startedAt, &finishedAt, &dep.ErrorMessage,
		&clusterID, &displayName, &arch, &nodes, &mysqlVersion, &configJSON,
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
	if clusterID.Valid {
		dep.ClusterID = clusterID.String
	}
	if displayName.Valid {
		dep.DisplayName = displayName.String
	}
	if arch.Valid {
		dep.Arch = arch.String
	}
	if nodes.Valid {
		dep.Nodes = int(nodes.Int64)
	}
	if mysqlVersion.Valid {
		dep.MySQLVersion = mysqlVersion.String
	}
	if configJSON.Valid {
		dep.ConfigJSON = configJSON.String
	}
	return &dep, nil
}

func (r *ClusterDeployRepository) getByIDInDB(ctx context.Context, id string) (*models.ClusterDeployment, error) {
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, cluster_id, display_name, arch, nodes, mysql_version, config_json, created_at, updated_at FROM cluster_deployments WHERE id = ?`
	return r.scanClusterDeployment(r.db.Pool.QueryRowContext(ctx, query, id))
}

func (r *ClusterDeployRepository) getByNameInDB(ctx context.Context, name string) (*models.ClusterDeployment, error) {
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, cluster_id, display_name, arch, nodes, mysql_version, config_json, created_at, updated_at FROM cluster_deployments WHERE name = ? ORDER BY created_at DESC LIMIT 1`
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

func (r *ClusterDeployRepository) GetByClusterID(ctx context.Context, clusterID string) (*models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, cluster_type, name, status, request_json, plan_json, custom_json, started_at, finished_at, error_message, cluster_id, display_name, arch, nodes, mysql_version, config_json, created_at, updated_at FROM cluster_deployments WHERE cluster_id = ? ORDER BY created_at DESC LIMIT 1`
	return r.scanClusterDeployment(r.db.Pool.QueryRowContext(ctx, query, clusterID))
}

func (r *ClusterDeployRepository) ListClusters(ctx context.Context) ([]models.ClusterDeployment, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT d.id, d.cluster_type, d.name, d.status, d.request_json, d.plan_json, d.custom_json,
		       d.started_at, d.finished_at, d.error_message, d.cluster_id, d.display_name, d.arch,
		       d.nodes, d.mysql_version, d.config_json, d.created_at, d.updated_at
		FROM cluster_deployments d
		INNER JOIN (
			SELECT cluster_id, MAX(created_at) AS latest_created_at
			FROM cluster_deployments
			WHERE cluster_id != '' AND status IN ('completed','success','running','in_progress')
			GROUP BY cluster_id
		) latest ON latest.cluster_id = d.cluster_id AND latest.latest_created_at = d.created_at
		WHERE d.cluster_id != '' AND d.status IN ('completed','success','running','in_progress')
		ORDER BY d.created_at DESC`
	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	defer rows.Close()

	deployments := make([]models.ClusterDeployment, 0)
	for rows.Next() {
		var dep models.ClusterDeployment
		var startedAt, finishedAt sql.NullTime
		var clusterID, displayName, arch, mysqlVersion, configJSON sql.NullString
		var nodes sql.NullInt64
		if err := rows.Scan(&dep.ID, &dep.ClusterType, &dep.Name, &dep.Status,
			&dep.RequestJSON, &dep.PlanJSON, &dep.CustomJSON,
			&startedAt, &finishedAt, &dep.ErrorMessage,
			&clusterID, &displayName, &arch, &nodes, &mysqlVersion, &configJSON,
			&dep.CreatedAt, &dep.UpdatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			dep.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			dep.FinishedAt = &finishedAt.Time
		}
		if clusterID.Valid {
			dep.ClusterID = clusterID.String
		}
		if displayName.Valid {
			dep.DisplayName = displayName.String
		}
		if arch.Valid {
			dep.Arch = arch.String
		}
		if nodes.Valid {
			dep.Nodes = int(nodes.Int64)
		}
		if mysqlVersion.Valid {
			dep.MySQLVersion = mysqlVersion.String
		}
		if configJSON.Valid {
			dep.ConfigJSON = configJSON.String
		}
		deployments = append(deployments, dep)
	}
	return deployments, rows.Err()
}

func (r *ClusterDeployRepository) UpdateClusterMeta(ctx context.Context, clusterID string, nodes int, mysqlVersion, configJSON string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE cluster_deployments SET nodes = ?, mysql_version = ?, config_json = ?, updated_at = ? WHERE cluster_id = ?`,
		nodes, mysqlVersion, configJSON, now, clusterID)
	if err != nil {
		return fmt.Errorf("failed to update cluster meta: %w", err)
	}
	return nil
}

func isFinalStatus(status string) bool {
	switch status {
	case "completed", "success", "failed", "destroyed", "cancelled":
		return true
	default:
		return false
	}
}
