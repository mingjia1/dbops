package services

import (
	"context"
	"fmt"
	"time"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type AuditService struct {
	auditRepo      *repositories.AuditLogRepository
	approvalRepo   *repositories.ApprovalRequestRepository
}

func NewAuditService(auditRepo *repositories.AuditLogRepository, approvalRepo *repositories.ApprovalRequestRepository) *AuditService {
	return &AuditService{
		auditRepo:    auditRepo,
		approvalRepo: approvalRepo,
	}
}

func (s *AuditService) CreateAuditLog(ctx context.Context, req CreateAuditLogRequest) (*models.AuditLog, error) {
	auditLog := &models.AuditLog{
		UserID:        req.UserID,
		Operation:     req.Operation,
		ResourceType:  req.ResourceType,
		ResourceID:    req.ResourceID,
		Action:        req.Action,
		Details:       req.Details,
		Result:        req.Result,
		ErrorMsg:      req.ErrorMsg,
		IPAddress:     req.IPAddress,
		UserAgent:     req.UserAgent,
		CreatedAt:     time.Now(),
	}

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return auditLog, nil
}

func (s *AuditService) GetAuditLogByID(ctx context.Context, id string) (*models.AuditLog, error) {
	return s.auditRepo.GetByID(ctx, id)
}

func (s *AuditService) ListAuditLogs(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	return s.auditRepo.List(ctx, limit, offset)
}

func (s *AuditService) ListAuditLogsByUser(ctx context.Context, userID string, limit, offset int) ([]models.AuditLog, error) {
	return s.auditRepo.ListByUserID(ctx, userID, limit, offset)
}

func (s *AuditService) ListAuditLogsByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]models.AuditLog, error) {
	return s.auditRepo.ListByResource(ctx, resourceType, resourceID, limit, offset)
}

type CreateAuditLogRequest struct {
	UserID        string `json:"user_id" binding:"required"`
	Operation     string `json:"operation" binding:"required"`
	ResourceType  string `json:"resource_type" binding:"required"`
	ResourceID    string `json:"resource_id"`
	Action        string `json:"action" binding:"required"`
	Details       string `json:"details"`
	Result        string `json:"result" binding:"required"`
	ErrorMsg      string `json:"error_msg"`
	IPAddress     string `json:"ip_address"`
	UserAgent     string `json:"user_agent"`
}