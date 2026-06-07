package models

import (
	"time"
)

type UpgradePlan struct {
	ID               string    `json:"id"`
	InstanceID       string    `json:"instance_id"`
	SourceVersion    string    `json:"source_version"`
	TargetVersion    string    `json:"target_version"`
	Strategy         string    `json:"strategy"`
	Status           string    `json:"status"`
	PreCheckResult   string    `json:"pre_check_result"`
	UpgradePath      string    `json:"upgrade_path"`
	EstimatedTime    int       `json:"estimated_time"`
	RiskLevel        string    `json:"risk_level"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
}

type UpgradeStep struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	StepOrder   int       `json:"step_order"`
	StepName    string    `json:"step_name"`
	StepType    string    `json:"step_type"`
	Status      string    `json:"status"`
	Command     string    `json:"command"`
	Output      string    `json:"output"`
	ErrorMsg    string    `json:"error_msg"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

type UpgradeBackup struct {
	ID             string    `json:"id"`
	PlanID         string    `json:"plan_id"`
	BackupType     string    `json:"backup_type"`
	BackupPath     string    `json:"backup_path"`
	BackupSize     int64     `json:"backup_size"`
	Checksum       string    `json:"checksum"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type UpgradeReport struct {
	ID                 string    `json:"id"`
	PlanID             string    `json:"plan_id"`
	ReportType         string    `json:"report_type"`
	Summary            string    `json:"summary"`
	Details            string    `json:"details"`
	PerformanceMetrics string    `json:"performance_metrics"`
	Issues             string    `json:"issues"`
	Recommendations    string    `json:"recommendations"`
	CreatedAt          time.Time `json:"created_at"`
}

type CompatibilityCheck struct {
	ID              string    `json:"id"`
	InstanceID      string    `json:"instance_id"`
	SourceVersion   string    `json:"source_version"`
	TargetVersion   string    `json:"target_version"`
	CheckType       string    `json:"check_type"`
	Status          string    `json:"status"`
	IsCompatible    bool      `json:"is_compatible"`
	WarningCount    int       `json:"warning_count"`
	ErrorCount      int       `json:"error_count"`
	CheckResult     string    `json:"check_result"`
	IncompatibilityList string `json:"incompatibility_list"`
	CheckedAt       time.Time `json:"checked_at"`
	CreatedAt       time.Time `json:"created_at"`
}