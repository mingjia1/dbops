package repositories

import (
	"context"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type TopologyEventRepository struct {
	db *Database
}

func NewTopologyEventRepository(db *Database) *TopologyEventRepository {
	return &TopologyEventRepository{db: db}
}

func (r *TopologyEventRepository) Create(ctx context.Context, event *models.TopologyEvent) error {
	_, err := r.db.Pool.ExecContext(ctx,
		`INSERT INTO topology_events (id, cluster_id, event_type, old_master_id, new_master_id, node_id, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.ClusterID, event.EventType, event.OldMasterID, event.NewMasterID, event.NodeID, event.Details, event.CreatedAt,
	)
	return err
}

func (r *TopologyEventRepository) ListByCluster(ctx context.Context, clusterID string, limit int) ([]models.TopologyEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, cluster_id, event_type, old_master_id, new_master_id, node_id, details, created_at
		 FROM topology_events WHERE cluster_id = ? ORDER BY created_at DESC LIMIT ?`,
		clusterID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.TopologyEvent
	for rows.Next() {
		var e models.TopologyEvent
		if err := rows.Scan(&e.ID, &e.ClusterID, &e.EventType, &e.OldMasterID, &e.NewMasterID, &e.NodeID, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *TopologyEventRepository) LatestByCluster(ctx context.Context, clusterID string) (*models.TopologyEvent, error) {
	row := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, cluster_id, event_type, old_master_id, new_master_id, node_id, details, created_at
		 FROM topology_events WHERE cluster_id = ? ORDER BY created_at DESC LIMIT 1`,
		clusterID,
	)
	var e models.TopologyEvent
	if err := row.Scan(&e.ID, &e.ClusterID, &e.EventType, &e.OldMasterID, &e.NewMasterID, &e.NodeID, &e.Details, &e.CreatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *TopologyEventRepository) DeleteByCluster(ctx context.Context, clusterID string) error {
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM topology_events WHERE cluster_id = ?`, clusterID)
	return err
}
