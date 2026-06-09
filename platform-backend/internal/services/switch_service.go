package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type SwitchService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	clusterRepo *repositories.ClusterDeployRepository
	agentClient *AgentClient
	// P1: 角色切换历史持久化, 之前是 in-memory map 重启即丢.
	historyRepo *repositories.RoleSwitchHistoryRepository

	mu      sync.RWMutex
	history map[string][]RoleSwitchRecord
}

func NewSwitchService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, clusterRepo *repositories.ClusterDeployRepository, agentClient *AgentClient, historyRepo *repositories.RoleSwitchHistoryRepository) *SwitchService {
	return &SwitchService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		clusterRepo: clusterRepo,
		historyRepo: historyRepo,
		agentClient: agentClient,
		history:     make(map[string][]RoleSwitchRecord),
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

type RoleSwitchRequest struct {
	ClusterID       string `json:"cluster_id" binding:"required"`
	InstanceID      string `json:"instance_id" binding:"required"`
	TargetRole      string `json:"target_role" binding:"required"`
	OldMasterID     string `json:"old_master_id"`
	ReplicationUser string `json:"replication_user"`
	ReplicationPass string `json:"replication_pass"`
	Force           bool   `json:"force"`
	RequireApproval bool   `json:"require_approval"`
	GracePeriodSec  int    `json:"grace_period_sec"`
}

type RoleSwitchResult struct {
	TaskID          string    `json:"task_id"`
	ClusterID       string    `json:"cluster_id"`
	ClusterType     string    `json:"cluster_type"`
	InstanceID      string    `json:"instance_id"`
	InstanceHost    string    `json:"instance_host"`
	OldRole         string    `json:"old_role"`
	NewRole         string    `json:"new_role"`
	OldMasterID     string    `json:"old_master_id"`
	NewMasterID     string    `json:"new_master_id"`
	RebuiltReplicas []string  `json:"rebuilt_replicas"`
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
}

// RoleSwitchRecord 别名到 models.RoleSwitchRecord, 保持向后兼容 (controllers/前端用的就是这个类型).
type RoleSwitchRecord = models.RoleSwitchRecord

func (s *SwitchService) SingleToMHA(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "mha")
}

func (s *SwitchService) SingleToMGR(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "mgr")
}

func (s *SwitchService) SingleToPXC(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error) {
	return s.executeSwitch(ctx, req, "standalone", "pxc")
}

func (s *SwitchService) executeSwitch(ctx context.Context, req SwitchClusterRequest, sourceType, targetType string) (*SwitchClusterResult, error) {
	inst, err := s.instRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	hostInfo, err := s.resolveAgentHost(ctx, inst)
	if err != nil {
		return nil, err
	}

	config := map[string]interface{}{
		"switch_type":  fmt.Sprintf("%s-to-%s", sourceType, targetType),
		"instance_id":  req.InstanceID,
		"vip":          req.VIP,
		"cluster_name": req.ClusterName,
	}

	result, err := s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/cluster-switch", map[string]interface{}{
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

func (s *SwitchService) SwitchRoleWithinCluster(ctx context.Context, req RoleSwitchRequest) (*RoleSwitchResult, error) {
	startedAt := time.Now()

	clusterType, err := s.detectClusterType(ctx, req.ClusterID)
	if err != nil {
		result := s.failedResult(req, "", "unknown", startedAt, fmt.Sprintf("failed to detect cluster architecture: %v", err))
		s.recordHistory(ctx, result)
		return result, nil
	}

	if !isValidRoleForCluster(clusterType, req.TargetRole) {
		result := s.failedResult(req, clusterType, "unknown", startedAt,
			fmt.Sprintf("target role %q is not valid for cluster architecture %q", req.TargetRole, clusterType))
		s.recordHistory(ctx, result)
		return result, nil
	}

	inst, err := s.instRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		result := s.failedResult(req, clusterType, "unknown", startedAt, fmt.Sprintf("instance not found: %v", err))
		s.recordHistory(ctx, result)
		return result, nil
	}

	if inst.ClusterID == "" || inst.ClusterID != req.ClusterID {
		result := s.failedResult(req, clusterType, "unknown", startedAt, "instance does not belong to the specified cluster")
		s.recordHistory(ctx, result)
		return result, nil
	}

	hostInfo, err := s.resolveAgentHost(ctx, inst)
	if err != nil {
		result := s.failedResult(req, clusterType, "unknown", startedAt, err.Error())
		s.recordHistory(ctx, result)
		return result, nil
	}

	currentRole, oldMasterID := s.guessCurrentRole(inst, clusterType)
	if currentRole == req.TargetRole {
		result := &RoleSwitchResult{
			ClusterID:    req.ClusterID,
			ClusterType:  clusterType,
			InstanceID:   req.InstanceID,
			InstanceHost: fmt.Sprintf("%s:%d", hostInfo.Address, hostInfo.Port),
			OldRole:      currentRole,
			NewRole:      req.TargetRole,
			OldMasterID:  oldMasterID,
			Status:       "skipped",
			Message:      "instance is already in the target role",
			StartedAt:    startedAt,
			CompletedAt:  time.Now(),
		}
		s.recordHistory(ctx, result)
		return result, nil
	}

	result := s.buildResultSkeleton(req, clusterType, inst, hostInfo, currentRole, oldMasterID, startedAt)
	if inst.Status.ReplicationStatus == "pseudo" {
		return s.switchPseudoRole(ctx, req, result, clusterType, currentRole)
	}

	demotingFromMaster := isPrimaryRole(currentRole)
	promotingToMaster := isPrimaryRole(req.TargetRole)

	if promotingToMaster && demotingFromMaster && req.InstanceID != req.OldMasterID && req.OldMasterID != "" {
		result = s.failedResultFromSkeleton(result, fmt.Sprintf("ambiguous old_master_id: %s vs instance %s", req.OldMasterID, req.InstanceID))
		s.recordHistory(ctx, result)
		return result, nil
	}

	if promotingToMaster {
		oldMasterID := req.OldMasterID
		if oldMasterID == "" {
			oldMasterID = result.OldMasterID
		}
		if oldMasterID != "" && oldMasterID != req.InstanceID {
			if err := s.demoteOldMaster(ctx, req.ClusterID, oldMasterID, clusterType); err != nil {
				result.Status = "failed"
				result.Message = fmt.Sprintf("demote old master failed: %v", err)
				result.CompletedAt = time.Now()
				s.recordHistory(ctx, result)
				return result, nil
			}
		}

		if err := s.promoteInstance(ctx, hostInfo, clusterType, req); err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("promote instance failed: %v", err)
			result.CompletedAt = time.Now()
			s.recordHistory(ctx, result)
			return result, nil
		}

		replicas, _ := s.rebuildReplicaTopology(ctx, req.ClusterID, req.InstanceID, oldMasterID, clusterType)
		result.RebuiltReplicas = replicas
		result.NewMasterID = req.InstanceID
		result.OldMasterID = oldMasterID
	} else {
		if err := s.demoteInstance(ctx, hostInfo, clusterType, req); err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("demote instance failed: %v", err)
			result.CompletedAt = time.Now()
			s.recordHistory(ctx, result)
			return result, nil
		}
	}

	result.Status = "completed"
	result.CompletedAt = time.Now()
	result.Message = fmt.Sprintf("role switched within %s cluster: %s -> %s", clusterType, currentRole, req.TargetRole)
	s.recordHistory(ctx, result)
	return result, nil
}

func (s *SwitchService) buildResultSkeleton(req RoleSwitchRequest, clusterType string, inst *models.Instance, hostInfo *AgentHostInfo, currentRole, oldMasterID string, startedAt time.Time) *RoleSwitchResult {
	return &RoleSwitchResult{
		TaskID:       uuid.New().String(),
		ClusterID:    req.ClusterID,
		ClusterType:  clusterType,
		InstanceID:   req.InstanceID,
		InstanceHost: fmt.Sprintf("%s:%d", hostInfo.Address, hostInfo.Port),
		OldRole:      currentRole,
		NewRole:      req.TargetRole,
		OldMasterID:  oldMasterID,
		StartedAt:    startedAt,
	}
}

func (s *SwitchService) failedResult(req RoleSwitchRequest, clusterType, currentRole string, startedAt time.Time, msg string) *RoleSwitchResult {
	return &RoleSwitchResult{
		ClusterID:   req.ClusterID,
		ClusterType: clusterType,
		InstanceID:  req.InstanceID,
		OldRole:     currentRole,
		NewRole:     req.TargetRole,
		Status:      "failed",
		Message:     msg,
		StartedAt:   startedAt,
		CompletedAt: time.Now(),
	}
}

func (s *SwitchService) failedResultFromSkeleton(r *RoleSwitchResult, msg string) *RoleSwitchResult {
	r.Status = "failed"
	r.Message = msg
	r.CompletedAt = time.Now()
	return r
}

type AgentHostInfo struct {
	Address string
	Port    int
}

func (s *SwitchService) resolveAgentHost(ctx context.Context, inst *models.Instance) (*AgentHostInfo, error) {
	if inst.HostID == nil || *inst.HostID == "" {
		return nil, fmt.Errorf("cannot determine host for instance %s", inst.ID)
	}

	host, err := s.hostRepo.GetByID(ctx, *inst.HostID)
	if err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	port := host.AgentPort
	if port <= 0 {
		port = 9090
	}
	return &AgentHostInfo{Address: host.Address, Port: port}, nil
}

func (s *SwitchService) detectClusterType(ctx context.Context, clusterID string) (string, error) {
	dep, err := s.clusterRepo.GetByID(ctx, clusterID)
	if err == nil && dep != nil {
		return dep.ClusterType, nil
	}
	return "", fmt.Errorf("cluster %s not found", clusterID)
}

func (s *SwitchService) guessCurrentRole(inst *models.Instance, clusterType string) (string, string) {
	if inst == nil {
		return "unknown", ""
	}
	if inst.Status.Role != "" && inst.Status.Role != "unknown" {
		return inst.Status.Role, inst.Topology.MasterID
	}
	return replicaRoleFor(clusterType), ""
}

func (s *SwitchService) promoteInstance(ctx context.Context, host *AgentHostInfo, clusterType string, req RoleSwitchRequest) error {
	config := map[string]interface{}{
		"cluster_id":       req.ClusterID,
		"instance_id":      req.InstanceID,
		"target_role":      req.TargetRole,
		"old_master_id":    req.OldMasterID,
		"replication_user": req.ReplicationUser,
		"replication_pass": req.ReplicationPass,
		"force":            req.Force,
		"grace_period_sec": req.GracePeriodSec,
		"cluster_type":     clusterType,
	}

	_, err := s.agentClient.callAgent(ctx, host.Address, host.Port, "/agent/tasks/role-promote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": req.InstanceID,
		"cluster_id":  req.ClusterID,
		"config":      config,
	})
	return err
}

func (s *SwitchService) demoteInstance(ctx context.Context, host *AgentHostInfo, clusterType string, req RoleSwitchRequest) error {
	config := map[string]interface{}{
		"cluster_id":       req.ClusterID,
		"instance_id":      req.InstanceID,
		"target_role":      req.TargetRole,
		"replication_user": req.ReplicationUser,
		"replication_pass": req.ReplicationPass,
		"force":            req.Force,
		"cluster_type":     clusterType,
	}

	_, err := s.agentClient.callAgent(ctx, host.Address, host.Port, "/agent/tasks/role-demote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": req.InstanceID,
		"cluster_id":  req.ClusterID,
		"config":      config,
	})
	return err
}

func (s *SwitchService) demoteOldMaster(ctx context.Context, clusterID, oldMasterID, clusterType string) error {
	inst, err := s.instRepo.GetByID(ctx, oldMasterID)
	if err != nil {
		return fmt.Errorf("old master instance not found: %w", err)
	}
	hostInfo, err := s.resolveAgentHost(ctx, inst)
	if err != nil {
		return err
	}
	config := map[string]interface{}{
		"cluster_id":   clusterID,
		"instance_id":  oldMasterID,
		"target_role":  replicaRoleFor(clusterType),
		"force":        true,
		"cluster_type": clusterType,
	}

	_, err = s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/role-demote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": oldMasterID,
		"cluster_id":  clusterID,
		"config":      config,
	})
	return err
}

func (s *SwitchService) rebuildReplicaTopology(ctx context.Context, clusterID, newMasterID, oldMasterID, clusterType string) ([]string, error) {
	allInsts, err := s.listClusterInstances(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	rebuilt := make([]string, 0, len(allInsts))
	for _, inst := range allInsts {
		if inst.ID == newMasterID || inst.ID == oldMasterID {
			continue
		}
		hostInfo, err := s.resolveAgentHost(ctx, inst)
		if err != nil {
			continue
		}

		config := map[string]interface{}{
			"cluster_id":    clusterID,
			"instance_id":   inst.ID,
			"new_master_id": newMasterID,
			"old_master_id": oldMasterID,
			"target_role":   replicaRoleFor(clusterType),
			"cluster_type":  clusterType,
		}

		_, err = s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/role-replica-rebuild", map[string]interface{}{
			"task_id":     uuid.New().String(),
			"instance_id": inst.ID,
			"cluster_id":  clusterID,
			"config":      config,
		})
		if err == nil {
			rebuilt = append(rebuilt, inst.ID)
		}
	}
	return rebuilt, nil
}

func (s *SwitchService) listClusterInstances(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	if s.instRepo == nil {
		return nil, nil
	}
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	for i, inst := range instances {
		full, err := s.instRepo.GetByID(ctx, inst.ID)
		if err == nil {
			instances[i] = full
		}
	}
	return instances, nil
}

func (s *SwitchService) switchPseudoRole(ctx context.Context, req RoleSwitchRequest, result *RoleSwitchResult, clusterType, currentRole string) (*RoleSwitchResult, error) {
	instances, err := s.listClusterInstances(ctx, req.ClusterID)
	if err != nil {
		return s.failedResultFromSkeleton(result, fmt.Sprintf("load pseudo cluster instances failed: %v", err)), nil
	}
	if len(instances) == 0 {
		return s.failedResultFromSkeleton(result, "pseudo cluster has no managed instances"), nil
	}

	newMasterID := result.OldMasterID
	if isPrimaryRole(req.TargetRole) {
		newMasterID = req.InstanceID
	}

	replicaIDs := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst.ID != newMasterID {
			replicaIDs = append(replicaIDs, inst.ID)
		}
	}

	slaveIDs, _ := json.Marshal(replicaIDs)
	for _, inst := range instances {
		role := replicaRoleFor(clusterType)
		masterID := newMasterID
		slaveIDText := ""

		if inst.ID == req.InstanceID {
			role = req.TargetRole
		}
		if inst.ID == newMasterID {
			masterID = ""
			slaveIDText = string(slaveIDs)
		}

		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			RunStatus:           defaultString(inst.Status.RunStatus, "running"),
			HealthStatus:        defaultString(inst.Status.HealthStatus, "healthy"),
			Role:                role,
			ReplicationStatus:   "pseudo",
			SecondsBehindMaster: 0,
		}); err != nil {
			return s.failedResultFromSkeleton(result, fmt.Sprintf("update pseudo instance status failed: %v", err)), nil
		}
		if err := s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       req.ClusterID,
			MasterID:        masterID,
			SlaveIDs:        slaveIDText,
			ReplicationMode: clusterType,
		}); err != nil {
			return s.failedResultFromSkeleton(result, fmt.Sprintf("update pseudo topology failed: %v", err)), nil
		}
	}

	result.Status = "completed"
	result.CompletedAt = time.Now()
	result.NewMasterID = newMasterID
	if isPrimaryRole(req.TargetRole) {
		result.RebuiltReplicas = replicaIDs
	}
	result.Message = fmt.Sprintf("pseudo role switched within %s cluster: %s -> %s", clusterType, currentRole, req.TargetRole)
	s.recordHistory(ctx, result)
	return result, nil
}

func (s *SwitchService) recordHistory(ctx context.Context, r *RoleSwitchResult) {
	record := &RoleSwitchRecord{
		ID:              uuid.New().String(),
		ClusterID:       r.ClusterID,
		ClusterType:     r.ClusterType,
		InstanceID:      r.InstanceID,
		InstanceHost:    r.InstanceHost,
		OldRole:         r.OldRole,
		NewRole:         r.NewRole,
		OldMasterID:     r.OldMasterID,
		NewMasterID:     r.NewMasterID,
		RebuiltReplicas: r.RebuiltReplicas,
		Status:          r.Status,
		Message:         r.Message,
		StartedAt:       r.StartedAt,
		CompletedAt:     r.CompletedAt,
	}

	// 优先落库, 失败也只是 log + 内存继续; 不阻塞业务流.
	if s.historyRepo != nil {
		if err := s.historyRepo.Create(ctx, record); err != nil {
			// 落库失败也保留内存, 后端不会因此报错
			// (用户体感: 重启后历史可能丢, 但本次操作不中断)
		}
	}
	s.mu.Lock()
	s.history[r.ClusterID] = append(s.history[r.ClusterID], *record)
	s.mu.Unlock()
}

func (s *SwitchService) ListRoleSwitchHistory(ctx context.Context, clusterID string, limit int) ([]RoleSwitchRecord, error) {
	// P1: 优先从 role_switch_history 表读 (跨重启持久化).
	// 表读失败时 fallback 到 in-memory (后端刚启动 / repo 不可用).
	if s.historyRepo != nil {
		if records, err := s.historyRepo.ListByCluster(ctx, clusterID, limit, 0); err == nil && len(records) > 0 {
			return records, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.history[clusterID]
	out := make([]RoleSwitchRecord, 0, len(records))
	out = append(out, records...)

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func isValidRoleForCluster(clusterType, role string) bool {
	switch clusterType {
	case "ha", "mha":
		return role == "master" || role == "slave" || role == "replica"
	case "mgr":
		return role == "primary" || role == "secondary" || role == "primary_master"
	case "pxc":
		return role == "primary" || role == "secondary" || role == "donor" || role == "joiner"
	default:
		return false
	}
}

func isPrimaryRole(role string) bool {
	return role == "master" || role == "primary" || role == "primary_master"
}

func replicaRoleFor(clusterType string) string {
	switch clusterType {
	case "ha", "mha":
		return "slave"
	case "mgr":
		return "secondary"
	case "pxc":
		return "secondary"
	default:
		return "replica"
	}
}
