package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type SwitchService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
}

func NewSwitchService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, agentClient *AgentClient) *SwitchService {
	return &SwitchService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		agentClient: agentClient,
	}
}

type SwitchClusterRequest struct {
	InstanceID  string `json:"instance_id" binding:"required"`
	TargetType  string `json:"target_type" binding:"required"`
	ClusterName string `json:"cluster_name"`
	VIP         string `json:"vip"`
}

type SwitchClusterResult struct {
	TaskID      string    `json:"task_id"`
	InstanceID  string    `json:"instance_id"`
	SourceType  string    `json:"source_type"`
	TargetType  string    `json:"target_type"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	CompletedAt time.Time `json:"completed_at"`
}

func (s *SwitchService) SingleToMHA(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "mha")
}

func (s *SwitchService) SingleToMGR(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "mgr")
}

func (s *SwitchService) SingleToPXC(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "pxc")
}

func (s *SwitchService) MHAToMGR(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "mha", "mgr")
}

func (s *SwitchService) MGRToPXC(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "mgr", "pxc")
}

func (s *SwitchService) PXCToMHA(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "pxc", "mha")
}

func (s *SwitchService) executeSwitch(ctx context.Context, req SwitchClusterRequest, sourceType, targetType string) (*SwitchClusterResult, error) {
	inst, err := s.instRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	var agentHost string
	agentPort := 9090
	if inst.HostID != nil && *inst.HostID != "" {
		host, err := s.hostRepo.GetByID(ctx, *inst.HostID)
		if err == nil {
			agentHost = host.Address
		}
	}
	if agentHost == "" {
		return nil, fmt.Errorf("cannot determine agent host for instance %s", req.InstanceID)
	}

	config := map[string]interface{}{
		"switch_type":  fmt.Sprintf("%s-to-%s", sourceType, targetType),
		"instance_id":  req.InstanceID,
		"vip":          req.VIP,
		"cluster_name": req.ClusterName,
	}

	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/cluster-switch", map[string]interface{}{
		"task_id":     req.InstanceID,
		"instance_id": req.InstanceID,
		"config":      config,
	})
	if err != nil {
		return &SwitchClusterResult{
			InstanceID: req.InstanceID,
			SourceType: sourceType,
			TargetType: targetType,
			Status:     "failed",
			Message:    fmt.Sprintf("Switch failed: %v", err),
		}, nil
	}

	return &SwitchClusterResult{
		InstanceID:  req.InstanceID,
		SourceType:  sourceType,
		TargetType:  targetType,
		Status:      result.Status,
		Message:     result.Message,
		CompletedAt: time.Now(),
	}, nil
}
