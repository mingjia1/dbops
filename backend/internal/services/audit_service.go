package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	// ID 必须在计算 hash 之前确定: payload 包含 log.ID, 若先算 hash 再由 repo
	// 生成 ID, 存储的 hash 将永远与重算结果不一致, 全链验证必然失败。
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
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

// VerifyChain: 全链完整性验证 —— 从链头逐条重算 HMAC 并校验 prev_hash 链接。
// 审计链为 hash 链(每条 prev_hash = 前一条 hash), 与条目 id 无关,
// 因此按时间正序遍历全表逐条校验, 而不是按 id 回溯。
// 返回 message(人类可读摘要) 与 details(结构化校验详情, 供前端展示具体表现)。
func (s *AuditService) VerifyChain(ctx context.Context) (bool, string, map[string]interface{}, error) {
	// 大 limit 一次取全链 (List 按时间倒序返回, 末位是最旧条目)
	logs, err := s.auditRepo.List(ctx, 1000000, 0)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	if len(logs) == 0 {
		return true, "链为空, 无可校验条目", map[string]interface{}{
			"total_entries": 0,
			"algorithm":     "HMAC-SHA256",
			"verified_at":   time.Now().Format(time.RFC3339),
		}, nil
	}

	oldest := logs[len(logs)-1]
	newest := logs[0]
	prevHash := ""
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		if log.PrevHash != prevHash {
			details := s.chainDetails(len(logs), &oldest, &newest)
			details["broken_entry_id"] = log.ID
			details["broken_entry_time"] = log.CreatedAt.Format(time.RFC3339)
			details["broken_reason"] = "prev_hash 不匹配"
			details["expected_prev_hash"] = prevHash
			details["claimed_prev_hash"] = log.PrevHash
			return false, fmt.Sprintf("prev_hash mismatch at entry %s (%s): claims %s, expected %s",
				log.ID, log.CreatedAt.Format(time.RFC3339), log.PrevHash, prevHash), details, nil
		}
		expected := s.computeAuditHash(log.PrevHash, &log)
		if log.Hash != expected {
			details := s.chainDetails(len(logs), &oldest, &newest)
			details["broken_entry_id"] = log.ID
			details["broken_entry_time"] = log.CreatedAt.Format(time.RFC3339)
			details["broken_reason"] = "hash 重算不匹配(条目内容与签名不符)"
			details["stored_hash"] = log.Hash
			details["recomputed_hash"] = expected
			return false, fmt.Sprintf("hash mismatch at entry %s: stored %s, recomputed %s",
				log.ID, log.Hash, expected), details, nil
		}
		prevHash = log.Hash
	}
	details := s.chainDetails(len(logs), &oldest, &newest)
	details["head_hash"] = newest.Hash
	details["tail_hash"] = oldest.Hash
	return true, fmt.Sprintf("全链校验通过: 共 %d 条审计日志逐条重算 HMAC 并校验链式链接, 均未被篡改", len(logs)), details, nil
}

// chainDetails: 汇总链的范围信息(条目数/时间跨度/首末条目摘要/算法/验证时刻)。
func (s *AuditService) chainDetails(total int, oldest, newest *models.AuditLog) map[string]interface{} {
	return map[string]interface{}{
		"total_entries":     total,
		"algorithm":         "HMAC-SHA256",
		"verified_at":       time.Now().Format(time.RFC3339),
		"oldest_entry_id":   oldest.ID,
		"oldest_entry_time": oldest.CreatedAt.Format(time.RFC3339),
		"oldest_action":     oldest.Action,
		"newest_entry_id":   newest.ID,
		"newest_entry_time": newest.CreatedAt.Format(time.RFC3339),
		"newest_action":     newest.Action,
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
