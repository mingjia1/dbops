package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

type FaultService struct {
	tplRepo  *repositories.FaultTemplateRepository
	execRepo *repositories.FaultExecutionRepository
}

func NewFaultService(tplRepo *repositories.FaultTemplateRepository, execRepo *repositories.FaultExecutionRepository) *FaultService {
	return &FaultService{tplRepo: tplRepo, execRepo: execRepo}
}

// --- Fault Templates ---

func (s *FaultService) CreateTemplate(ctx context.Context, ft *models.FaultTemplate) error {
	ft.CreatedAt = time.Now()
	ft.UpdatedAt = time.Now()
	return s.tplRepo.Create(ctx, ft)
}

func (s *FaultService) ListTemplates(ctx context.Context, category string) ([]models.FaultTemplate, error) {
	return s.tplRepo.List(ctx, category)
}

func (s *FaultService) GetTemplate(ctx context.Context, id string) (*models.FaultTemplate, error) {
	return s.tplRepo.GetByID(ctx, id)
}

func (s *FaultService) DeleteTemplate(ctx context.Context, id string) error {
	return s.tplRepo.Delete(ctx, id)
}

// --- Fault Execution ---

type ExecuteFaultRequest struct {
	TemplateID string `json:"template_id" binding:"required"`
	DrillID    string `json:"drill_id"`
	TargetType string `json:"target_type" binding:"required"`
	TargetID   string `json:"target_id" binding:"required"`
}

func (s *FaultService) ExecuteFault(ctx context.Context, req ExecuteFaultRequest) (*models.FaultExecution, error) {
	tpl, err := s.tplRepo.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	now := time.Now()
	exec := &models.FaultExecution{
		TemplateID: req.TemplateID,
		DrillID:    req.DrillID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		FaultType:  tpl.FaultType,
		Params:     tpl.Params,
		Status:     "injecting",
		StartedAt:  now,
		CreatedAt:  now,
	}

	if err := s.execRepo.Create(ctx, exec); err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	result, err := s.injectFault(ctx, tpl, exec)
	if err != nil {
		exec.Status = "failed"
		exec.Result = err.Error()
		now2 := time.Now()
		exec.CompletedAt = &now2
		_ = s.execRepo.Update(ctx, exec)
		return exec, fmt.Errorf("fault injection failed: %w", err)
	}

	exec.Status = "active"
	exec.Result = result
	_ = s.execRepo.Update(ctx, exec)
	return exec, nil
}

func (s *FaultService) injectFault(ctx context.Context, tpl *models.FaultTemplate, exec *models.FaultExecution) (string, error) {
	switch tpl.FaultType {
	case "network_partition":
		return s.injectNetworkPartition(ctx, exec)
	case "node_crash":
		return s.injectNodeCrash(ctx, exec)
	case "disk_full":
		return s.injectDiskFull(ctx, exec)
	case "io_hang":
		return s.injectIOHang(ctx, exec)
	case "connection_exhaust":
		return s.injectConnectionExhaust(ctx, exec)
	case "slow_query_flood":
		return s.injectSlowQueryFlood(ctx, exec)
	default:
		// Generic fault — record params as result
		detail := map[string]interface{}{
			"fault_type": tpl.FaultType,
			"target":     exec.TargetID,
			"params":     tpl.Params,
			"simulated":  true,
		}
		b, _ := json.Marshal(detail)
		return string(b), nil
	}
}

func (s *FaultService) injectNetworkPartition(ctx context.Context, exec *models.FaultExecution) (string, error) {
	// In production, this would configure iptables/nftables on the host via agent
	params := map[string]interface{}{
		"fault":       "network_partition",
		"target":      exec.TargetID,
		"action":      "block_inbound",
		"duration_sec": 30,
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

func (s *FaultService) injectNodeCrash(ctx context.Context, exec *models.FaultExecution) (string, error) {
	params := map[string]interface{}{
		"fault":       "node_crash",
		"target":      exec.TargetID,
		"action":      "kill_mysql_process",
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

func (s *FaultService) injectDiskFull(ctx context.Context, exec *models.FaultExecution) (string, error) {
	params := map[string]interface{}{
		"fault":       "disk_full",
		"target":      exec.TargetID,
		"fill_percent": 95,
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

func (s *FaultService) injectIOHang(ctx context.Context, exec *models.FaultExecution) (string, error) {
	params := map[string]interface{}{
		"fault":       "io_hang",
		"target":      exec.TargetID,
		"delay_ms":    5000,
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

func (s *FaultService) injectConnectionExhaust(ctx context.Context, exec *models.FaultExecution) (string, error) {
	params := map[string]interface{}{
		"fault":       "connection_exhaust",
		"target":      exec.TargetID,
		"max_connections": 10,
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

func (s *FaultService) injectSlowQueryFlood(ctx context.Context, exec *models.FaultExecution) (string, error) {
	params := map[string]interface{}{
		"fault":       "slow_query_flood",
		"target":      exec.TargetID,
		"concurrent_queries": 50,
		"simulated":   true,
	}
	b, _ := json.Marshal(params)
	return string(b), nil
}

// RollbackFault marks a fault execution as rolled back.
func (s *FaultService) RollbackFault(ctx context.Context, id string) (*models.FaultExecution, error) {
	exec, err := s.execRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	exec.Status = "rolled_back"
	exec.RollbackAt = &now
	if err := s.execRepo.Update(ctx, exec); err != nil {
		return nil, err
	}
	return exec, nil
}

func (s *FaultService) ListExecutions(ctx context.Context, drillID string) ([]models.FaultExecution, error) {
	return s.execRepo.List(ctx, drillID)
}

func (s *FaultService) GetExecution(ctx context.Context, id string) (*models.FaultExecution, error) {
	return s.execRepo.GetByID(ctx, id)
}
