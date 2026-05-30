package models

import (
	"time"
)

type UpgradePlan struct {
	ID               string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	InstanceID       string    `json:"instance_id" gorm:"type:varchar(64);index;not null"`
	SourceVersion    string    `json:"source_version" gorm:"type:varchar(32);not null"`
	TargetVersion    string    `json:"target_version" gorm:"type:varchar(32);not null"`
	Strategy         string    `json:"strategy" gorm:"type:varchar(32);not null"`
	Status           string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	PreCheckResult   string    `json:"pre_check_result" gorm:"type:text"`
	UpgradePath      string    `json:"upgrade_path" gorm:"type:text"`
	EstimatedTime    int       `json:"estimated_time" gorm:"type:int"`
	RiskLevel        string    `json:"risk_level" gorm:"type:varchar(16)"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	StartedAt        time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt      time.Time `json:"completed_at" gorm:"type:timestamp"`
}

type UpgradeStep struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	PlanID      string    `json:"plan_id" gorm:"type:varchar(64);index;not null"`
	StepOrder   int       `json:"step_order" gorm:"type:int;not null"`
	StepName    string    `json:"step_name" gorm:"type:varchar(128);not null"`
	StepType    string    `json:"step_type" gorm:"type:varchar(32);not null"`
	Status      string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	Command     string    `json:"command" gorm:"type:text"`
	Output      string    `json:"output" gorm:"type:text"`
	ErrorMsg    string    `json:"error_msg" gorm:"type:text"`
	StartedAt   time.Time `json:"started_at" gorm:"type:timestamp"`
	CompletedAt time.Time `json:"completed_at" gorm:"type:timestamp"`
}

type UpgradeBackup struct {
	ID             string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	PlanID         string    `json:"plan_id" gorm:"type:varchar(64);index;not null"`
	BackupType     string    `json:"backup_type" gorm:"type:varchar(32);not null"`
	BackupPath     string    `json:"backup_path" gorm:"type:varchar(512)"`
	BackupSize     int64     `json:"backup_size" gorm:"type:bigint"`
	Checksum       string    `json:"checksum" gorm:"type:varchar(128)"`
	Status         string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type UpgradeReport struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	PlanID             string    `json:"plan_id" gorm:"type:varchar(64);index;not null"`
	ReportType         string    `json:"report_type" gorm:"type:varchar(32);not null"`
	Summary            string    `json:"summary" gorm:"type:text"`
	Details            string    `json:"details" gorm:"type:text"`
	PerformanceMetrics string    `json:"performance_metrics" gorm:"type:text"`
	Issues             string    `json:"issues" gorm:"type:text"`
	Recommendations    string    `json:"recommendations" gorm:"type:text"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type CompatibilityCheck struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	InstanceID      string    `json:"instance_id" gorm:"type:varchar(64);index;not null"`
	SourceVersion   string    `json:"source_version" gorm:"type:varchar(32);not null"`
	TargetVersion   string    `json:"target_version" gorm:"type:varchar(32);not null"`
	CheckType       string    `json:"check_type" gorm:"type:varchar(32);not null"`
	Status          string    `json:"status" gorm:"type:varchar(32);default:'pending'"`
	IsCompatible    bool      `json:"is_compatible" gorm:"type:boolean"`
	WarningCount    int       `json:"warning_count" gorm:"type:int;default:0"`
	ErrorCount      int       `json:"error_count" gorm:"type:int;default:0"`
	CheckResult     string    `json:"check_result" gorm:"type:text"`
	IncompatibilityList string `json:"incompatibility_list" gorm:"type:text"`
	CheckedAt       time.Time `json:"checked_at" gorm:"type:timestamp"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}