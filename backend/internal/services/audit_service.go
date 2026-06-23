package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

type AuditService struct {
	auditRepo    *repositories.AuditLogRepository
	approvalRepo *repositories.ApprovalRequestRepository
	hmacSecret   string
}

func NewAuditService(auditRepo *repositories.AuditLogRepository, approvalRepo *repositories.ApprovalRequestRepository) *AuditService {
	return &AuditService{
		auditRepo:    auditRepo,
		approvalRepo: approvalRepo,
	}
}

func (s *AuditService) SetHMACSecret(secret string) {
	s.hmacSecret = secret
}

func (s *AuditService) computeAuditHash(prevHash string, log *models.AuditLog) string {
	mac := hmac.New(sha256.New, []byte(s.hmacSecret))
	payload := strings.Join([]string{
		log.ID, log.UserID, log.Operation, log.ResourceType, log.ResourceID,
		log.Action, log.Details, log.Result, log.ErrorMsg, log.IPAddress, log.UserAgent,
		prevHash, log.CreatedAt.Format(time.RFC3339Nano),
	}, "|")
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AuditService) CreateAuditLog(ctx context.Context, req CreateAuditLogRequest) (*models.AuditLog, error) {
	prevHash, err := s.auditRepo.GetLatestHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest hash: %w", err)
	}

	auditLog := &models.AuditLog{
		UserID:       req.UserID,
		Operation:    req.Operation,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		Action:       req.Action,
		Details:      req.Details,
		Result:       req.Result,
		ErrorMsg:     req.ErrorMsg,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		PrevHash:     prevHash,
		CreatedAt:    time.Now(),
	}
	auditLog.Hash = s.computeAuditHash(prevHash, auditLog)

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

type FilteredResult struct {
	Logs  []models.AuditLog `json:"logs"`
	Total int               `json:"total"`
}

func (s *AuditService) ListAuditLogsFiltered(ctx context.Context, filter repositories.AuditLogFilter, limit, offset int) (*FilteredResult, error) {
	logs, total, err := s.auditRepo.ListFiltered(ctx, filter, limit, offset)
	if err != nil {
		return nil, err
	}
	return &FilteredResult{Logs: logs, Total: total}, nil
}

func (s *AuditService) VerifyChain(ctx context.Context, fromID string) (bool, string, error) {
	log, err := s.auditRepo.GetByID(ctx, fromID)
	if err != nil {
		return false, "", fmt.Errorf("audit log not found: %s", fromID)
	}

	currentHash := log.Hash
	for {
		if log.PrevHash == "" {
			return true, "chain intact", nil
		}
		prev, err := s.auditRepo.GetByID(ctx, log.PrevHash)
		if err != nil {
			return false, "", fmt.Errorf("broken chain at %s: previous entry %s not found", log.ID, log.PrevHash)
		}
		expectedHash := s.computeAuditHash(prev.PrevHash, prev)
		if prev.Hash != expectedHash {
			return false, fmt.Sprintf("hash mismatch at %s", prev.ID), nil
		}
		if prev.Hash != currentHash {
			return false, fmt.Sprintf("prev_hash mismatch: %s claims %s but actual is %s", log.ID, log.PrevHash, prev.Hash), nil
		}
		if prev.PrevHash == "" {
			return true, "chain intact", nil
		}
		log = prev
		currentHash = prev.Hash
	}
}

type CreateAuditLogRequest struct {
	UserID       string `json:"user_id" binding:"required"`
	Operation    string `json:"operation" binding:"required"`
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action" binding:"required"`
	Details      string `json:"details"`
	Result       string `json:"result" binding:"required"`
	ErrorMsg     string `json:"error_msg"`
	IPAddress    string `json:"ip_address"`
	UserAgent    string `json:"user_agent"`
}
