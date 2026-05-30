package models

import (
	"time"
)

type MigrationStrategy string

const (
	MigrationStrategyPhysical    MigrationStrategy = "physical"
	MigrationStrategyReplication MigrationStrategy = "replication"
	MigrationStrategyGTID        MigrationStrategy = "gtid"
)

type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusPreparing  MigrationStatus = "preparing"
	MigrationStatusMigrating  MigrationStatus = "migrating"
	MigrationStatusVerifying  MigrationStatus = "verifying"
	MigrationStatusSwitching  MigrationStatus = "switching"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusCancelled  MigrationStatus = "cancelled"
)

type MigrationTask struct {
	ID               string           `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name             string           `json:"name" gorm:"type:varchar(128);not null"`
	SourceInstanceID string           `json:"source_instance_id" gorm:"type:varchar(64);not null;index"`
	TargetInstanceID string           `json:"target_instance_id" gorm:"type:varchar(64);not null;index"`
	Strategy         MigrationStrategy `json:"strategy" gorm:"type:varchar(32);not null"`
	Status           MigrationStatus  `json:"status" gorm:"type:varchar(32);default:'pending'"`
	Progress         int              `json:"progress" gorm:"type:int;default:0"`
	
	StartedAt       *time.Time        `json:"started_at" gorm:"type:timestamp"`
	CompletedAt     *time.Time        `json:"completed_at" gorm:"type:timestamp"`
	
	Config          string            `json:"config" gorm:"type:text"`
	Error           string            `json:"error" gorm:"type:text"`
	
	CreatedAt       time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	
	Steps []MigrationStep `json:"steps" gorm:"foreignKey:TaskID"`
}

type MigrationStep struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TaskID      string    `json:"task_id" gorm:"type:varchar(64);not null;index"`
	Name        string    `json:"name" gorm:"type:varchar(64);not null"`
	Status      string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	StartedAt   *time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt *time.Time `json:"completed_at" gorm:"type:timestamp"`
	Error       string    `json:"error" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type MigrationProgress struct {
	TaskID           string    `json:"task_id"`
	Status           MigrationStatus `json:"status"`
	Progress         int       `json:"progress"`
	CurrentStep      string    `json:"current_step"`
	TotalSteps       int       `json:"total_steps"`
	CompletedSteps   int       `json:"completed_steps"`
	DataTransferred  int64     `json:"data_transferred"`
	EstimatedTime    int       `json:"estimated_time"`
	StartedAt        time.Time `json:"started_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type MigrationVerification struct {
	TaskID           string    `json:"task_id"`
	SourceCount      int64     `json:"source_count"`
	TargetCount      int64     `json:"target_count"`
	DataConsistency  bool      `json:"data_consistency"`
	SchemaMatch      bool      `json:"schema_match"`
	ChecksumMatch    bool      `json:"checksum_match"`
	VerifiedAt       time.Time `json:"verified_at"`
	Errors           []string  `json:"errors"`
}

type MigrationSwitchResult struct {
	TaskID           string    `json:"task_id"`
	OldMaster        string    `json:"old_master"`
	NewMaster        string    `json:"new_master"`
	SwitchedAt       time.Time `json:"switched_at"`
	ApplicationUpdated bool    `json:"application_updated"`
	Downtime         int64     `json:"downtime_ms"`
	Status           string    `json:"status"`
}