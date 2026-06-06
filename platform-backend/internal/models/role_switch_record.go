package models

import "time"

// RoleSwitchRecord P1: 之前在 services/switch_service.go 内部,
// 加 role_switch_history 表后挪到 models 让 repository 也用.
type RoleSwitchRecord struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	ClusterID       string    `json:"cluster_id" gorm:"type:varchar(64);index"`
	ClusterType     string    `json:"cluster_type" gorm:"type:varchar(32)"`
	InstanceID      string    `json:"instance_id" gorm:"type:varchar(64)"`
	InstanceHost    string    `json:"instance_host" gorm:"type:varchar(255)"`
	OldRole         string    `json:"old_role" gorm:"type:varchar(32)"`
	NewRole         string    `json:"new_role" gorm:"type:varchar(32)"`
	OldMasterID     string    `json:"old_master_id" gorm:"type:varchar(64)"`
	NewMasterID     string    `json:"new_master_id" gorm:"type:varchar(64)"`
	RebuiltReplicas []string  `json:"rebuilt_replicas" gorm:"-"`
	Status          string    `json:"status" gorm:"type:varchar(32)"`
	Message         string    `json:"message" gorm:"type:text"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
}
