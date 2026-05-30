package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type AuditLogRepository struct {
	db *Database
}

func NewAuditLogRepository(db *Database) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(ctx context.Context, auditLog *models.AuditLog) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	
	auditLog.ID = uuid.New().String()
	
	query := `
		INSERT INTO audit_logs (id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
		auditLog.ID, auditLog.UserID, auditLog.Operation, auditLog.ResourceType, auditLog.ResourceID,
		auditLog.Action, auditLog.Details, auditLog.Result, auditLog.ErrorMsg, auditLog.IPAddress,
		auditLog.UserAgent, auditLog.CreatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	
	return nil
}

func (r *AuditLogRepository) GetByID(ctx context.Context, id string) (*models.AuditLog, error) {
	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, created_at
		FROM audit_logs WHERE id = $1
	`
	
	auditLog := &models.AuditLog{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&auditLog.ID, &auditLog.UserID, &auditLog.Operation, &auditLog.ResourceType, &auditLog.ResourceID,
		&auditLog.Action, &auditLog.Details, &auditLog.Result, &auditLog.ErrorMsg, &auditLog.IPAddress,
		&auditLog.UserAgent, &auditLog.CreatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("audit log not found")
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}
	
	return auditLog, nil
}

func (r *AuditLogRepository) List(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.AuditLog{}, nil
	}
	
	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()
	
	var auditLogs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}
	
	return auditLogs, nil
}

func (r *AuditLogRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, created_at
		FROM audit_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	
	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by user: %w", err)
	}
	defer rows.Close()
	
	var auditLogs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}
	
	return auditLogs, nil
}

func (r *AuditLogRepository) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, created_at
		FROM audit_logs WHERE resource_type = $1 AND resource_id = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4
	`
	
	rows, err := r.db.Pool.Query(ctx, query, resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by resource: %w", err)
	}
	defer rows.Close()
	
	var auditLogs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}
	
	return auditLogs, nil
}

type ApprovalRequestRepository struct {
	db *Database
}

func NewApprovalRequestRepository(db *Database) *ApprovalRequestRepository {
	return &ApprovalRequestRepository{db: db}
}

func (r *ApprovalRequestRepository) Create(ctx context.Context, approvalRequest *models.ApprovalRequest) error {
	approvalRequest.ID = uuid.New().String()
	
	query := `
		INSERT INTO approval_requests (id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
		approvalRequest.ID, approvalRequest.RequesterID, approvalRequest.ApproverID, approvalRequest.OperationType,
		approvalRequest.ResourceType, approvalRequest.ResourceID, approvalRequest.RequestReason,
		approvalRequest.ApprovalStatus, approvalRequest.ApprovalComment, approvalRequest.Priority,
		approvalRequest.ExpiresAt, approvalRequest.CreatedAt, approvalRequest.UpdatedAt, approvalRequest.ApprovedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create approval request: %w", err)
	}
	
	return nil
}

func (r *ApprovalRequestRepository) GetByID(ctx context.Context, id string) (*models.ApprovalRequest, error) {
	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests WHERE id = $1
	`
	
	approvalRequest := &models.ApprovalRequest{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&approvalRequest.ID, &approvalRequest.RequesterID, &approvalRequest.ApproverID, &approvalRequest.OperationType,
		&approvalRequest.ResourceType, &approvalRequest.ResourceID, &approvalRequest.RequestReason,
		&approvalRequest.ApprovalStatus, &approvalRequest.ApprovalComment, &approvalRequest.Priority,
		&approvalRequest.ExpiresAt, &approvalRequest.CreatedAt, &approvalRequest.UpdatedAt, &approvalRequest.ApprovedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("approval request not found")
		}
		return nil, fmt.Errorf("failed to get approval request: %w", err)
	}
	
	return approvalRequest, nil
}

func (r *ApprovalRequestRepository) List(ctx context.Context, limit, offset int) ([]models.ApprovalRequest, error) {
	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests ORDER BY priority DESC, created_at DESC LIMIT $1 OFFSET $2
	`
	
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list approval requests: %w", err)
	}
	defer rows.Close()
	
	var approvalRequests []models.ApprovalRequest
	for rows.Next() {
		var req models.ApprovalRequest
		if err := rows.Scan(&req.ID, &req.RequesterID, &req.ApproverID, &req.OperationType,
			&req.ResourceType, &req.ResourceID, &req.RequestReason,
			&req.ApprovalStatus, &req.ApprovalComment, &req.Priority,
			&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.ApprovedAt); err != nil {
			return nil, err
		}
		approvalRequests = append(approvalRequests, req)
	}
	
	return approvalRequests, nil
}

func (r *ApprovalRequestRepository) ListByStatus(ctx context.Context, status string, limit, offset int) ([]models.ApprovalRequest, error) {
	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests WHERE approval_status = $1 ORDER BY priority DESC, created_at DESC LIMIT $2 OFFSET $3
	`
	
	rows, err := r.db.Pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list approval requests by status: %w", err)
	}
	defer rows.Close()
	
	var approvalRequests []models.ApprovalRequest
	for rows.Next() {
		var req models.ApprovalRequest
		if err := rows.Scan(&req.ID, &req.RequesterID, &req.ApproverID, &req.OperationType,
			&req.ResourceType, &req.ResourceID, &req.RequestReason,
			&req.ApprovalStatus, &req.ApprovalComment, &req.Priority,
			&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt, &req.ApprovedAt); err != nil {
			return nil, err
		}
		approvalRequests = append(approvalRequests, req)
	}
	
	return approvalRequests, nil
}

func (r *ApprovalRequestRepository) Update(ctx context.Context, approvalRequest *models.ApprovalRequest) error {
	query := `
		UPDATE approval_requests 
		SET approver_id = $2, approval_status = $3, approval_comment = $4, updated_at = $5, approved_at = $6
		WHERE id = $1
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
		approvalRequest.ID, approvalRequest.ApproverID, approvalRequest.ApprovalStatus,
		approvalRequest.ApprovalComment, approvalRequest.UpdatedAt, approvalRequest.ApprovedAt)
	
	if err != nil {
		return fmt.Errorf("failed to update approval request: %w", err)
	}
	
	return nil
}

func (r *ApprovalRequestRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM approval_requests WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete approval request: %w", err)
	}
	return nil
}