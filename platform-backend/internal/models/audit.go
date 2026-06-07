package models

import (
	"time"
)

type AuditLog struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Operation   string    `json:"operation"`
	ResourceType string   `json:"resource_type"`
	ResourceID  string    `json:"resource_id"`
	Action      string    `json:"action"`
	Details     string    `json:"details"`
	Result      string    `json:"result"`
	ErrorMsg    string    `json:"error_msg"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

type ApprovalRequest struct {
	ID            string    `json:"id"`
	RequesterID   string    `json:"requester_id"`
	ApproverID    string    `json:"approver_id"`
	OperationType string    `json:"operation_type"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    string    `json:"resource_id"`
	RequestReason string    `json:"request_reason"`
	ApprovalStatus string   `json:"approval_status"`
	ApprovalComment string  `json:"approval_comment"`
	Priority      int       `json:"priority"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ApprovedAt    time.Time `json:"approved_at"`
}

type HighRiskOperation struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	RiskLevel   string    `json:"risk_level"`
	RequiresApproval bool  `json:"requires_approval"`
	ApprovalTimeout int   `json:"approval_timeout"`
	AffectedResources string `json:"affected_resources"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}