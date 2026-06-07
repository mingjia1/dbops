package models

import "time"

// RoleSwitchRecord P1: 之前在 services/switch_service.go 内部,
// 加 role_switch_history 表后挪到 models 让 repository 也用.
type RoleSwitchRecord struct {
	ID              string    `json:"id"`
	ClusterID       string    `json:"cluster_id"`
	ClusterType     string    `json:"cluster_type"`
	InstanceID      string    `json:"instance_id"`
	InstanceHost    string    `json:"instance_host"`
	OldRole         string    `json:"old_role"`
	NewRole         string    `json:"new_role"`
	OldMasterID     string    `json:"old_master_id"`
	NewMasterID     string    `json:"new_master_id"`
	RebuiltReplicas []string  `json:"rebuilt_replicas"`
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
}
