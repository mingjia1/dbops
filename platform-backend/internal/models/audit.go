package models

import (
	"time"
)

type AuditLog struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	UserID      string    `json:"user_id" gorm:"type:varchar(64);index;not null"`
	Operation   string    `json:"operation" gorm:"type:varchar(64);not null"`
	ResourceType string   `json:"resource_type" gorm:"type:varchar(64);not null"`
	ResourceID  string    `json:"resource_id" gorm:"type:varchar(64);index"`
	Action      string    `json:"action" gorm:"type:varchar(32);not null"`
	Details     string    `json:"details" gorm:"type:text"`
	Result      string    `json:"result" gorm:"type:varchar(32);not null"`
	ErrorMsg    string    `json:"error_msg" gorm:"type:text"`
	IPAddress   string    `json:"ip_address" gorm:"type:varchar(64)"`
	UserAgent   string    `json:"user_agent" gorm:"type:varchar(255)"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type ApprovalRequest struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RequesterID   string    `json:"requester_id" gorm:"type:varchar(64);index;not null"`
	ApproverID    string    `json:"approver_id" gorm:"type:varchar(64);index"`
	OperationType string    `json:"operation_type" gorm:"type:varchar(64);not null"`
	ResourceType  string    `json:"resource_type" gorm:"type:varchar(64);not null"`
	ResourceID    string    `json:"resource_id" gorm:"type:varchar(64);index;not null"`
	RequestReason string    `json:"request_reason" gorm:"type:text"`
	ApprovalStatus string   `json:"approval_status" gorm:"type:varchar(32);default:'pending';index"`
	ApprovalComment string  `json:"approval_comment" gorm:"type:text"`
	Priority      int       `json:"priority" gorm:"type:int;default:1"`
	ExpiresAt     time.Time `json:"expires_at" gorm:"type:timestamp"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	ApprovedAt    time.Time `json:"approved_at" gorm:"type:timestamp"`
}

type HighRiskOperation struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name        string    `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	Category    string    `json:"category" gorm:"type:varchar(64);not null"`
	Description string    `json:"description" gorm:"type:text"`
	RiskLevel   string    `json:"risk_level" gorm:"type:varchar(32);not null"`
	RequiresApproval bool  `json:"requires_approval" gorm:"type:boolean;default:true"`
	ApprovalTimeout int   `json:"approval_timeout" gorm:"type:int;default:3600"`
	AffectedResources string `json:"affected_resources" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}