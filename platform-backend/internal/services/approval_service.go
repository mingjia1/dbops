package services

import (
	"context"
	"fmt"
	"time"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type ApprovalService struct {
	approvalRepo *repositories.ApprovalRequestRepository
	auditRepo    *repositories.AuditLogRepository
}

func NewApprovalService(approvalRepo *repositories.ApprovalRequestRepository, auditRepo *repositories.AuditLogRepository) *ApprovalService {
	return &ApprovalService{
		approvalRepo: approvalRepo,
		auditRepo:    auditRepo,
	}
}

func (s *ApprovalService) CreateApprovalRequest(ctx context.Context, req CreateApprovalRequestRequest) (*models.ApprovalRequest, error) {
	approvalRequest := &models.ApprovalRequest{
		RequesterID:    req.RequesterID,
		OperationType:  req.OperationType,
		ResourceType:   req.ResourceType,
		ResourceID:     req.ResourceID,
		RequestReason:  req.RequestReason,
		ApprovalStatus: "pending",
		Priority:       req.Priority,
		ExpiresAt:      time.Now().Add(time.Duration(req.ExpiryHours) * time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.approvalRepo.Create(ctx, approvalRequest); err != nil {
		return nil, fmt.Errorf("failed to create approval request: %w", err)
	}

	return approvalRequest, nil
}

func (s *ApprovalService) GetApprovalRequestByID(ctx context.Context, id string) (*models.ApprovalRequest, error) {
	return s.approvalRepo.GetByID(ctx, id)
}

func (s *ApprovalService) ListApprovalRequests(ctx context.Context, limit, offset int) ([]models.ApprovalRequest, error) {
	return s.approvalRepo.List(ctx, limit, offset)
}

func (s *ApprovalService) ListApprovalRequestsByStatus(ctx context.Context, status string, limit, offset int) ([]models.ApprovalRequest, error) {
	return s.approvalRepo.ListByStatus(ctx, status, limit, offset)
}

func (s *ApprovalService) ApproveRequest(ctx context.Context, id, approverID, comment string) (*models.ApprovalRequest, error) {
	approvalRequest, err := s.approvalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if approvalRequest.ApprovalStatus != "pending" {
		return nil, fmt.Errorf("approval request is not in pending status")
	}

	now := time.Now()
	approvalRequest.ApproverID = approverID
	approvalRequest.ApprovalStatus = "approved"
	approvalRequest.ApprovalComment = comment
	approvalRequest.UpdatedAt = now
	approvalRequest.ApprovedAt = now

	if err := s.approvalRepo.Update(ctx, approvalRequest); err != nil {
		return nil, fmt.Errorf("failed to approve request: %w", err)
	}

	auditLog := &models.AuditLog{
		UserID:       approverID,
		Operation:    "approve_request",
		ResourceType: "approval_request",
		ResourceID:   id,
		Action:       "approve",
		Result:       "success",
		CreatedAt:    now,
	}
	s.auditRepo.Create(ctx, auditLog)

	return approvalRequest, nil
}

func (s *ApprovalService) RejectRequest(ctx context.Context, id, approverID, comment string) (*models.ApprovalRequest, error) {
	approvalRequest, err := s.approvalRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if approvalRequest.ApprovalStatus != "pending" {
		return nil, fmt.Errorf("approval request is not in pending status")
	}

	now := time.Now()
	approvalRequest.ApproverID = approverID
	approvalRequest.ApprovalStatus = "rejected"
	approvalRequest.ApprovalComment = comment
	approvalRequest.UpdatedAt = now
	approvalRequest.ApprovedAt = now

	if err := s.approvalRepo.Update(ctx, approvalRequest); err != nil {
		return nil, fmt.Errorf("failed to reject request: %w", err)
	}

	auditLog := &models.AuditLog{
		UserID:       approverID,
		Operation:    "reject_request",
		ResourceType: "approval_request",
		ResourceID:   id,
		Action:       "reject",
		Result:       "success",
		CreatedAt:    now,
	}
	s.auditRepo.Create(ctx, auditLog)

	return approvalRequest, nil
}

type CreateApprovalRequestRequest struct {
	RequesterID   string `json:"requester_id" binding:"required"`
	OperationType string `json:"operation_type" binding:"required"`
	ResourceType  string `json:"resource_type" binding:"required"`
	ResourceID    string `json:"resource_id" binding:"required"`
	RequestReason string `json:"request_reason"`
	Priority      int    `json:"priority"`
	ExpiryHours   int    `json:"expiry_hours"`
}