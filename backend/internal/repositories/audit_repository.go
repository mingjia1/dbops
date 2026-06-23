package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	query := `
		INSERT INTO audit_logs (id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		auditLog.ID, auditLog.UserID, auditLog.Operation, auditLog.ResourceType, auditLog.ResourceID,
		auditLog.Action, auditLog.Details, auditLog.Result, auditLog.ErrorMsg, auditLog.IPAddress,
		auditLog.UserAgent, auditLog.PrevHash, auditLog.Hash, auditLog.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

func (r *AuditLogRepository) GetByID(ctx context.Context, id string) (*models.AuditLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available in standalone mode")
	}

	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at
		FROM audit_logs WHERE id = ?
	`

	auditLog := &models.AuditLog{}
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&auditLog.ID, &auditLog.UserID, &auditLog.Operation, &auditLog.ResourceType, &auditLog.ResourceID,
		&auditLog.Action, &auditLog.Details, &auditLog.Result, &auditLog.ErrorMsg, &auditLog.IPAddress,
		&auditLog.UserAgent, &auditLog.PrevHash, &auditLog.Hash, &auditLog.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("audit log not found")
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	return auditLog, nil
}

func (r *AuditLogRepository) List(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	auditLogs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.PrevHash, &log.Hash, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}

	return auditLogs, nil
}

func (r *AuditLogRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]models.AuditLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at
		FROM audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by user: %w", err)
	}
	defer rows.Close()

	auditLogs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.PrevHash, &log.Hash, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}

	return auditLogs, nil
}

func (r *AuditLogRepository) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]models.AuditLog, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at
		FROM audit_logs WHERE resource_type = ? AND resource_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by resource: %w", err)
	}
	defer rows.Close()

	auditLogs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.PrevHash, &log.Hash, &log.CreatedAt); err != nil {
			return nil, err
		}
		auditLogs = append(auditLogs, log)
	}

	return auditLogs, nil
}

type AuditLogFilter struct {
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	StartTime    *time.Time
	EndTime      *time.Time
}

func buildAuditWhereClause(filter AuditLogFilter) []string {
	where := make([]string, 0, 6)
	if filter.UserID != "" {
		where = append(where, "LOWER(user_id) LIKE ?")
	}
	if filter.Action != "" {
		where = append(where, "(LOWER(action) LIKE ? OR LOWER(operation) LIKE ?)")
	}
	if filter.ResourceType != "" {
		where = append(where, "resource_type = ?")
	}
	if filter.ResourceID != "" {
		where = append(where, "resource_id = ?")
	}
	if filter.StartTime != nil {
		where = append(where, "created_at >= ?")
	}
	if filter.EndTime != nil {
		where = append(where, "created_at <= ?")
	}
	return where
}

func buildAuditFilterArgs(filter AuditLogFilter) []interface{} {
	args := make([]interface{}, 0, 8)
	if filter.UserID != "" {
		args = append(args, "%"+strings.ToLower(filter.UserID)+"%")
	}
	if filter.Action != "" {
		action := "%" + strings.ToLower(filter.Action) + "%"
		args = append(args, action, action)
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		args = append(args, filter.ResourceID)
	}
	if filter.StartTime != nil {
		args = append(args, filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		args = append(args, filter.EndTime.UTC())
	}
	return args
}

func (r *AuditLogRepository) ListFiltered(ctx context.Context, filter AuditLogFilter, limit, offset int) ([]models.AuditLog, int, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, 0, fmt.Errorf("database not available")
	}

	where := buildAuditWhereClause(filter)
	args := buildAuditFilterArgs(filter)

	total, err := r.CountFiltered(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, operation, resource_type, resource_id, action, details, result, error_msg, ip_address, user_agent, prev_hash, hash, created_at
		FROM audit_logs
	`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	allArgs := append(args, limit, offset)

	rows, err := r.db.Pool.QueryContext(ctx, query, allArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list filtered audit logs: %w", err)
	}
	defer rows.Close()

	auditLogs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Operation, &log.ResourceType, &log.ResourceID,
			&log.Action, &log.Details, &log.Result, &log.ErrorMsg, &log.IPAddress,
			&log.UserAgent, &log.PrevHash, &log.Hash, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		auditLogs = append(auditLogs, log)
	}
	return auditLogs, total, rows.Err()
}

func (r *AuditLogRepository) GetLatestHash(ctx context.Context) (string, error) {
	if r.db == nil || r.db.Pool == nil {
		return "", nil
	}
	var hash string
	err := r.db.Pool.QueryRowContext(ctx, `SELECT hash FROM audit_logs ORDER BY created_at DESC LIMIT 1`).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get latest hash: %w", err)
	}
	return hash, nil
}

func (r *AuditLogRepository) CountFiltered(ctx context.Context, filter AuditLogFilter) (int, error) {
	if r.db == nil || r.db.Pool == nil {
		return 0, nil
	}

	where := buildAuditWhereClause(filter)
	query := "SELECT COUNT(*) FROM audit_logs"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	args := buildAuditFilterArgs(filter)
	if err := r.db.Pool.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count audit logs: %w", err)
	}
	return total, nil
}

type ApprovalRequestRepository struct {
	db *Database
}

func NewApprovalRequestRepository(db *Database) *ApprovalRequestRepository {
	return &ApprovalRequestRepository{db: db}
}

func (r *ApprovalRequestRepository) Create(ctx context.Context, approvalRequest *models.ApprovalRequest) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	if approvalRequest.ID == "" {
		approvalRequest.ID = uuid.New().String()
	}

	query := `
		INSERT INTO approval_requests (id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
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
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available in standalone mode")
	}

	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests WHERE id = ?
	`

	approvalRequest := &models.ApprovalRequest{}
	var (
		approverID  sql.NullString
		approvalCmt sql.NullString
		expiresAt   sql.NullTime
		updatedAt   sql.NullTime
		approvedAt  sql.NullTime
	)
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&approvalRequest.ID, &approvalRequest.RequesterID, &approverID, &approvalRequest.OperationType,
		&approvalRequest.ResourceType, &approvalRequest.ResourceID, &approvalRequest.RequestReason,
		&approvalRequest.ApprovalStatus, &approvalCmt, &approvalRequest.Priority,
		&expiresAt, &approvalRequest.CreatedAt, &updatedAt, &approvedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("approval request not found")
		}
		return nil, fmt.Errorf("failed to get approval request: %w", err)
	}

	if approverID.Valid {
		approvalRequest.ApproverID = approverID.String
	}
	if approvalCmt.Valid {
		approvalRequest.ApprovalComment = approvalCmt.String
	}
	if expiresAt.Valid {
		approvalRequest.ExpiresAt = expiresAt.Time
	}
	if updatedAt.Valid {
		approvalRequest.UpdatedAt = updatedAt.Time
	}
	if approvedAt.Valid {
		approvalRequest.ApprovedAt = approvedAt.Time
	}

	return approvalRequest, nil
}

func (r *ApprovalRequestRepository) List(ctx context.Context, limit, offset int) ([]models.ApprovalRequest, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list approval requests: %w", err)
	}
	defer rows.Close()

	approvalRequests := make([]models.ApprovalRequest, 0)
	for rows.Next() {
		req, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		approvalRequests = append(approvalRequests, *req)
	}

	return approvalRequests, nil
}

func (r *ApprovalRequestRepository) ListByStatus(ctx context.Context, status string, limit, offset int) ([]models.ApprovalRequest, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, requester_id, approver_id, operation_type, resource_type, resource_id, request_reason, approval_status, approval_comment, priority, expires_at, created_at, updated_at, approved_at
		FROM approval_requests WHERE approval_status = ? ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list approval requests by status: %w", err)
	}
	defer rows.Close()

	approvalRequests := make([]models.ApprovalRequest, 0)
	for rows.Next() {
		req, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		approvalRequests = append(approvalRequests, *req)
	}

	return approvalRequests, nil
}

func (r *ApprovalRequestRepository) Update(ctx context.Context, approvalRequest *models.ApprovalRequest) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	now := time.Now()
	approvalRequest.UpdatedAt = now
	query := `
		UPDATE approval_requests 
		SET approver_id = ?, approval_status = ?, approval_comment = ?, updated_at = ?, approved_at = ?
		WHERE id = ?
	`

	res, err := r.db.Pool.ExecContext(ctx, query,
		approvalRequest.ApproverID, approvalRequest.ApprovalStatus,
		approvalRequest.ApprovalComment, approvalRequest.UpdatedAt, approvalRequest.ApprovedAt,
		approvalRequest.ID)

	if err != nil {
		return fmt.Errorf("failed to update approval request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check approval request update result: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("approval request not found")
	}

	return nil
}

func (r *ApprovalRequestRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	res, err := r.db.Pool.ExecContext(ctx, `DELETE FROM approval_requests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete approval request: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check approval request delete result: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("approval request not found")
	}
	return nil
}

func scanApprovalRequest(rows *sql.Rows) (*models.ApprovalRequest, error) {
	var (
		req         models.ApprovalRequest
		approverID  sql.NullString
		approvalCmt sql.NullString
		expiresAt   sql.NullTime
		updatedAt   sql.NullTime
		approvedAt  sql.NullTime
	)
	if err := rows.Scan(
		&req.ID, &req.RequesterID, &approverID, &req.OperationType,
		&req.ResourceType, &req.ResourceID, &req.RequestReason,
		&req.ApprovalStatus, &approvalCmt, &req.Priority,
		&expiresAt, &req.CreatedAt, &updatedAt, &approvedAt); err != nil {
		return nil, err
	}
	if approverID.Valid {
		req.ApproverID = approverID.String
	}
	if approvalCmt.Valid {
		req.ApprovalComment = approvalCmt.String
	}
	if expiresAt.Valid {
		req.ExpiresAt = expiresAt.Time
	}
	if updatedAt.Valid {
		req.UpdatedAt = updatedAt.Time
	}
	if approvedAt.Valid {
		req.ApprovedAt = approvedAt.Time
	}
	return &req, nil
}
