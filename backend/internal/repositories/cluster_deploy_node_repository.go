package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type ClusterDeployNodeRepository struct {
	db *Database
}

func NewClusterDeployNodeRepository(db *Database) *ClusterDeployNodeRepository {
	return &ClusterDeployNodeRepository{db: db}
}

func (r *ClusterDeployNodeRepository) Create(ctx context.Context, node *models.ClusterDeployNode) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	now := time.Now()
	if node.StartedAt == nil {
		node.StartedAt = &now
	}
	query := `INSERT INTO cluster_deploy_nodes (id, deployment_id, instance_id, host, mysql_port, role, status, current_step, message, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Pool.ExecContext(ctx, query, node.ID, node.DeploymentID, node.InstanceID, node.Host, node.MySQLPort, node.Role, node.Status, node.CurrentStep, node.Message, node.StartedAt, node.FinishedAt, node.ErrorMessage)
	if err != nil {
		return fmt.Errorf("failed to create deploy node: %w", err)
	}
	return nil
}

func (r *ClusterDeployNodeRepository) BatchCreate(ctx context.Context, nodes []models.ClusterDeployNode) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	tx, err := r.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO cluster_deploy_nodes (id, deployment_id, instance_id, host, mysql_port, role, status, current_step, message, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}
	defer stmt.Close()

	for i := range nodes {
		if nodes[i].ID == "" {
			nodes[i].ID = uuid.New().String()
		}
		if nodes[i].StartedAt == nil {
			nodes[i].StartedAt = &now
		}
		if _, err := stmt.ExecContext(ctx, nodes[i].ID, nodes[i].DeploymentID, nodes[i].InstanceID, nodes[i].Host, nodes[i].MySQLPort, nodes[i].Role, nodes[i].Status, nodes[i].CurrentStep, nodes[i].Message, nodes[i].StartedAt, nodes[i].FinishedAt, nodes[i].ErrorMessage); err != nil {
			return fmt.Errorf("failed to insert deploy node %s: %w", nodes[i].Host, err)
		}
	}
	return tx.Commit()
}

func (r *ClusterDeployNodeRepository) GetByID(ctx context.Context, id string) (*models.ClusterDeployNode, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, deployment_id, instance_id, host, mysql_port, role, status, current_step, message, started_at, finished_at, error_message FROM cluster_deploy_nodes WHERE id = ?`
	var node models.ClusterDeployNode
	var startedAt, finishedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(&node.ID, &node.DeploymentID, &node.InstanceID, &node.Host, &node.MySQLPort, &node.Role, &node.Status, &node.CurrentStep, &node.Message, &startedAt, &finishedAt, &node.ErrorMessage)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deploy node not found")
		}
		return nil, fmt.Errorf("failed to get deploy node: %w", err)
	}
	if startedAt.Valid {
		node.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		node.FinishedAt = &finishedAt.Time
	}
	return &node, nil
}

func (r *ClusterDeployNodeRepository) ListByDeploymentID(ctx context.Context, deploymentID string) ([]models.ClusterDeployNode, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `SELECT id, deployment_id, instance_id, host, mysql_port, role, status, current_step, message, started_at, finished_at, error_message FROM cluster_deploy_nodes WHERE deployment_id = ? ORDER BY started_at ASC`
	rows, err := r.db.Pool.QueryContext(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list deploy nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]models.ClusterDeployNode, 0)
	for rows.Next() {
		var node models.ClusterDeployNode
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&node.ID, &node.DeploymentID, &node.InstanceID, &node.Host, &node.MySQLPort, &node.Role, &node.Status, &node.CurrentStep, &node.Message, &startedAt, &finishedAt, &node.ErrorMessage); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			node.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			node.FinishedAt = &finishedAt.Time
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *ClusterDeployNodeRepository) UpdateStatus(ctx context.Context, id, status, currentStep, message string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	query := `UPDATE cluster_deploy_nodes SET status = ?, current_step = ?, message = ?, started_at = COALESCE(started_at, ?)`
	args := []interface{}{status, currentStep, message, now}
	if isFinalNodeStatus(status) {
		query += `, finished_at = ?`
		args = append(args, now)
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	_, err := r.db.Pool.ExecContext(ctx, query, args...)
	return err
}

func (r *ClusterDeployNodeRepository) UpdateStatusWithError(ctx context.Context, id, status, currentStep, message, errorMessage string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE cluster_deploy_nodes SET status = ?, current_step = ?, message = ?, error_message = ?, finished_at = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`,
		status, currentStep, message, errorMessage, now, now, id)
	return err
}

func (r *ClusterDeployNodeRepository) UpdateStatusByInstanceID(ctx context.Context, deploymentID, instanceID, status, currentStep, message, errorMessage string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	query := `UPDATE cluster_deploy_nodes SET status = ?, current_step = ?, message = ?, started_at = COALESCE(started_at, ?)`
	args := []interface{}{status, currentStep, message, now}
	if errorMessage != "" {
		query += `, error_message = ?`
		args = append(args, errorMessage)
	}
	if isFinalNodeStatus(status) {
		query += `, finished_at = ?`
		args = append(args, now)
	}
	query += ` WHERE deployment_id = ? AND instance_id = ?`
	args = append(args, deploymentID, instanceID)
	_, err := r.db.Pool.ExecContext(ctx, query, args...)
	return err
}

func (r *ClusterDeployNodeRepository) DeleteByDeploymentID(ctx context.Context, deploymentID string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM cluster_deploy_nodes WHERE deployment_id = ?`, deploymentID)
	return err
}

func (r *ClusterDeployNodeRepository) UpdateStatusByDeploymentID(ctx context.Context, deploymentID, status, message string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE cluster_deploy_nodes SET status = ?, message = ?, finished_at = ? WHERE deployment_id = ? AND status NOT IN ('completed', 'success', 'failed', 'error', 'cancelled')`,
		status, message, now, deploymentID)
	return err
}

func isFinalNodeStatus(status string) bool {
	switch status {
	case "completed", "success", "failed", "error", "cancelled":
		return true
	default:
		return false
	}
}
