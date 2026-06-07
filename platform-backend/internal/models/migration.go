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
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	SourceInstanceID string           `json:"source_instance_id"`
	TargetInstanceID string           `json:"target_instance_id"`
	Strategy         MigrationStrategy `json:"strategy"`
	Status           MigrationStatus  `json:"status"`
	Progress         int              `json:"progress"`
	
	StartedAt       *time.Time        `json:"started_at"`
	CompletedAt     *time.Time        `json:"completed_at"`
	
	Config          string            `json:"config"`
	Error           string            `json:"error"`
	
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	
	Steps []MigrationStep `json:"steps"`
}

type MigrationStep struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Error       string    `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
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