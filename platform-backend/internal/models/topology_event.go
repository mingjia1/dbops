package models

import "time"

type TopologyEvent struct {
	ID          string    `json:"id"`
	ClusterID   string    `json:"cluster_id"`
	EventType   string    `json:"event_type"`
	OldMasterID string    `json:"old_master_id"`
	NewMasterID string    `json:"new_master_id"`
	NodeID      string    `json:"node_id"`
	Details     string    `json:"details"`
	CreatedAt   time.Time `json:"created_at"`
}
