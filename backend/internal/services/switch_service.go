package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type SwitchService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	clusterRepo *repositories.ClusterDeployRepository
	agentClient *AgentClient
	auditSvc    *AuditService
	encKey      string
	// P1: role switch history persistence (previously in-memory map, lost on restart).
	historyRepo *repositories.RoleSwitchHistoryRepository

	logger  *log.Logger
	mu      sync.RWMutex
	history map[string][]RoleSwitchRecord
}

func NewSwitchService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, clusterRepo *repositories.ClusterDeployRepository, agentClient *AgentClient, historyRepo *repositories.RoleSwitchHistoryRepository, auditSvc ...*AuditService) *SwitchService {
	var audit *AuditService
	if len(auditSvc) > 0 {
		audit = auditSvc[0]
	}
	return &SwitchService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		clusterRepo: clusterRepo,
		historyRepo: historyRepo,
		agentClient: agentClient,
		auditSvc:    audit,
		logger:      log.Default(),
		history:     make(map[string][]RoleSwitchRecord),
	}
}

func (s *SwitchService) SetEncryptionKey(key string) {
	s.encKey = key
}

type SwitchClusterRequest struct {
	InstanceID       string   `json:"instance_id"`
	InstanceIDs      []string `json:"instance_ids"`
	TargetType       string   `json:"target_type"`
	ClusterID        string   `json:"cluster_id"`
	ClusterName      string   `json:"cluster_name"`
	VIP              string   `json:"vip"`
	ReplicationUser  string   `json:"replication_user"`
	ReplicationPass  string   `json:"replication_pass"`
	MySQLUser        string   `json:"mysql_user"`
	MySQLPassword    string   `json:"mysql_password"`
	ForceReconfigure bool     `json:"force_reconfigure"`
	// SkipSSH skips the SSH-dependent MHA/MGR/PXC deployment steps and only
	// registers the cluster topology metadata (roles, cluster_id).
	// Use this when SSH is not available and replication is already configured manually.
	SkipSSH bool `json:"skip_ssh"`
}

type SwitchClusterResult struct {
	TaskID      string    `json:"task_id"`
	ClusterID   string    `json:"cluster_id,omitempty"`
	InstanceID  string    `json:"instance_id"`
	InstanceIDs []string  `json:"instance_ids,omitempty"`
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

// RoleSwitchRecord is an alias for models.RoleSwitchRecord for backward compatibility.
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
	req.TargetType = defaultString(req.TargetType, targetType)
	if len(req.InstanceIDs) > 0 {
		return s.executeMultiInstanceSwitch(ctx, req, sourceType, targetType)
	}
	if req.InstanceID == "" {
		return nil, fmt.Errorf("instance_id or instance_ids is required")
	}
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

	if s.agentClient == nil {
		out := &SwitchClusterResult{
			InstanceID: req.InstanceID,
			SourceType: sourceType,
			TargetType: targetType,
			Status:     "failed",
			Message:    "agent client not configured",
		}
		s.auditClusterSwitch(ctx, req, out)
		return out, nil
	}
	result, err := s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/cluster-switch", map[string]interface{}{
		"task_id":     req.InstanceID,
		"instance_id": req.InstanceID,
		"config":      config,
	})
	if err != nil {
		out := &SwitchClusterResult{
			InstanceID: req.InstanceID,
			SourceType: sourceType,
			TargetType: targetType,
			Status:     "failed",
			Message:    fmt.Sprintf("Switch failed: %v", err),
		}
		s.auditClusterSwitch(ctx, req, out)
		return out, nil
	}

	out := &SwitchClusterResult{
		InstanceID:  req.InstanceID,
		SourceType:  sourceType,
		TargetType:  targetType,
		Status:      result.Status,
		Message:     result.Message,
		CompletedAt: time.Now(),
	}
	s.auditClusterSwitch(ctx, req, out)
	return out, nil
}

func (s *SwitchService) executeMultiInstanceSwitch(ctx context.Context, req SwitchClusterRequest, sourceType, targetType string) (*SwitchClusterResult, error) {
	if len(req.InstanceIDs) < 2 {
		return nil, fmt.Errorf("instance_ids must contain at least two instances for %s reconfiguration", targetType)
	}
	clusterID := defaultString(req.ClusterID, req.ClusterName)
	if clusterID == "" {
		clusterID = fmt.Sprintf("%s-%s", targetType, uuid.NewString())
	}
	clusterName := defaultString(req.ClusterName, clusterID)

	instances := make([]*models.Instance, 0, len(req.InstanceIDs))
	hosts := make([]*AgentHostInfo, 0, len(req.InstanceIDs))
	for _, id := range req.InstanceIDs {
		inst, err := s.instRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("instance %s not found: %w", id, err)
		}
		if inst.ClusterID != "" && inst.ClusterID != clusterID && !req.ForceReconfigure {
			return nil, fmt.Errorf("instance %s already belongs to cluster %s", id, inst.ClusterID)
		}
		hostInfo, err := s.resolveAgentHost(ctx, inst)
		if err != nil {
			return nil, fmt.Errorf("instance %s host resolution failed: %w", id, err)
		}
		instances = append(instances, inst)
		hosts = append(hosts, hostInfo)
	}

	if s.agentClient == nil {
		out := &SwitchClusterResult{
			ClusterID:   clusterID,
			InstanceIDs: req.InstanceIDs,
			SourceType:  sourceType,
			TargetType:  targetType,
			Status:      "failed",
			Message:     "agent client not configured",
			CompletedAt: time.Now(),
		}
		s.auditClusterSwitch(ctx, req, out)
		return out, nil
	}

	if err := s.ensureClusterDeployment(ctx, clusterID, clusterName, targetType); err != nil {
		return nil, err
	}

	nodeConfigs := make([]map[string]interface{}, 0, len(instances))
	for i, inst := range instances {
		conn, err := s.instRepo.GetConnection(ctx, inst.ID)
		if err != nil {
			return nil, fmt.Errorf("instance %s connection not found: %w", inst.ID, err)
		}
		role := replicaRoleFor(targetType)
		if i == 0 {
			role = primaryRoleFor(targetType)
		}
		node := map[string]interface{}{
			"instance_id": inst.ID,
			"host":        conn.Host,
			"port":        conn.Port,
			"role":        role,
			"agent_host":  hosts[i].Address,
			"agent_port":  hosts[i].Port,
		}
		if target, err := s.instanceConnectionConfig(ctx, inst.ID); err == nil {
			for k, v := range target {
				node[k] = v
			}
		}
		nodeConfigs = append(nodeConfigs, node)
	}

	// If SkipSSH is set, skip the SSH-dependent agent cluster-switch calls
	// and only register the cluster topology metadata.
	if req.SkipSSH {
		if err := s.applyClusterReconfigurationMetadata(ctx, clusterID, targetType, instances, req.MySQLUser, req.MySQLPassword); err != nil {
			out := &SwitchClusterResult{
				ClusterID:   clusterID,
				InstanceIDs: req.InstanceIDs,
				SourceType:  sourceType,
				TargetType:  targetType,
				Status:      "failed",
				Message:     fmt.Sprintf("metadata registration failed: %v", err),
				CompletedAt: time.Now(),
			}
			s.auditClusterSwitch(ctx, req, out)
			return out, nil
		}
		_ = s.clusterRepo.UpdateStatus(ctx, clusterID, "completed")
		out := &SwitchClusterResult{
			ClusterID:   clusterID,
			InstanceIDs: req.InstanceIDs,
			SourceType:  sourceType,
			TargetType:  targetType,
			Status:      "completed",
			Message:     fmt.Sprintf("%d instances registered as %s cluster (SSH skipped)", len(instances), targetType),
			CompletedAt: time.Now(),
		}
		s.auditClusterSwitch(ctx, req, out)
		return out, nil
	}

	completed := 0
	var lastMessage string
	for i, inst := range instances {
		config := map[string]interface{}{
			"switch_type":       fmt.Sprintf("%s-to-%s", sourceType, targetType),
			"cluster_id":        clusterID,
			"cluster_name":      clusterName,
			"cluster_type":      targetType,
			"target_type":       targetType,
			"instance_id":       inst.ID,
			"instance_ids":      req.InstanceIDs,
			"nodes":             nodeConfigs,
			"role":              nodeConfigs[i]["role"],
			"vip":               req.VIP,
			"replication_user":  req.ReplicationUser,
			"replication_pass":  req.ReplicationPass,
			"mysql_user":        req.MySQLUser,
			"mysql_password":    req.MySQLPassword,
			"force_reconfigure": req.ForceReconfigure,
		}
		if target, err := s.instanceConnectionConfig(ctx, inst.ID); err == nil {
			for k, v := range target {
				config[k] = v
			}
		}
		result, err := s.agentClient.callAgent(ctx, hosts[i].Address, hosts[i].Port, "/agent/tasks/cluster-switch", map[string]interface{}{
			"task_id":     uuid.NewString(),
			"instance_id": inst.ID,
			"cluster_id":  clusterID,
			"config":      config,
		})
		if err != nil {
			lastMessage = fmt.Sprintf("instance %s switch call failed: %v", inst.ID, err)
			continue
		}
		if taskErr := switchAgentTaskError("cluster switch", result); taskErr != nil {
			lastMessage = fmt.Sprintf("instance %s %v", inst.ID, taskErr)
			continue
		}
		completed++
		lastMessage = result.Message
	}

	status := "completed"
	if completed == 0 {
		status = "failed"
	} else if completed != len(instances) {
		status = "partial_success"
	}
	if completed == len(instances) {
		if err := s.applyClusterReconfigurationMetadata(ctx, clusterID, targetType, instances, req.MySQLUser, req.MySQLPassword); err != nil {
			status = "partial_success"
			lastMessage = fmt.Sprintf("cluster switched but metadata update failed: %v", err)
		} else {
			_ = s.clusterRepo.UpdateStatus(ctx, clusterID, "completed")
			lastMessage = fmt.Sprintf("%d standalone instances reconfigured as %s cluster", completed, targetType)
		}
	}

	out := &SwitchClusterResult{
		ClusterID:   clusterID,
		InstanceIDs: req.InstanceIDs,
		SourceType:  sourceType,
		TargetType:  targetType,
		Status:      status,
		Message:     lastMessage,
		CompletedAt: time.Now(),
	}
	s.auditClusterSwitch(ctx, req, out)
	return out, nil
}

// SwitchRoleWithinCluster switches the role of an instance within a cluster.
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

	if err := s.updateRealRoleTopology(ctx, req.ClusterID, result.NewMasterID, clusterType); err != nil {
		result.Status = "partial_success"
		result.Message = fmt.Sprintf("role switched but topology update failed: %v", err)
		result.CompletedAt = time.Now()
		s.recordHistory(ctx, result)
		return result, nil
	}

	result.Status = "completed"
	result.CompletedAt = time.Now()
	result.Message = fmt.Sprintf("role switched within %s cluster: %s -> %s", clusterType, currentRole, req.TargetRole)
	s.recordHistory(ctx, result)
	return result, nil
}

func (s *SwitchService) updateRealRoleTopology(ctx context.Context, clusterID, newMasterID, clusterType string) error {
	if newMasterID == "" {
		return nil
	}
	instances, err := s.listClusterInstances(ctx, clusterID)
	if err != nil {
		return err
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
		if inst.ID == newMasterID {
			role = primaryRoleFor(clusterType)
			masterID = ""
			slaveIDText = string(slaveIDs)
		}
		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			RunStatus:           defaultString(inst.Status.RunStatus, "running"),
			HealthStatus:        defaultString(inst.Status.HealthStatus, "healthy"),
			Role:                role,
			ReplicationStatus:   clusterType,
			SecondsBehindMaster: 0,
		}); err != nil {
			return err
		}
		if err := s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       clusterID,
			MasterID:        masterID,
			SlaveIDs:        slaveIDText,
			ReplicationMode: clusterType,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *SwitchService) ensureClusterDeployment(ctx context.Context, clusterID, clusterName, clusterType string) error {
	if s.clusterRepo == nil {
		return fmt.Errorf("cluster repository is not configured")
	}
	if dep, err := s.clusterRepo.GetByID(ctx, clusterID); err == nil && dep != nil {
		return s.clusterRepo.UpdateStatus(ctx, clusterID, "running")
	}
	return s.clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          clusterID,
		ClusterType: clusterType,
		Name:        clusterName,
		Status:      "running",
	})
}

func (s *SwitchService) applyClusterReconfigurationMetadata(ctx context.Context, clusterID, clusterType string, instances []*models.Instance, mysqlCreds ...string) error {
	if len(instances) == 0 {
		return nil
	}

	// Parse optional MySQL user/password from variadic args
	mysqlUser := ""
	mysqlPassword := ""
	if len(mysqlCreds) >= 2 {
		mysqlUser = mysqlCreds[0]
		mysqlPassword = mysqlCreds[1]
	}

	primaryID := instances[0].ID
	replicaIDs := make([]string, 0, len(instances)-1)
	for _, inst := range instances[1:] {
		replicaIDs = append(replicaIDs, inst.ID)
	}
	slaveIDs, _ := json.Marshal(replicaIDs)
	for i, inst := range instances {
		full := *inst
		full.ClusterID = clusterID
		if err := s.instRepo.Update(ctx, &full); err != nil {
			return fmt.Errorf("update instance %s cluster_id failed: %w", inst.ID, err)
		}

		// Store MySQL password in connection if provided
		if strings.TrimSpace(mysqlPassword) != "" && s.encKey != "" {
			enc, encErr := utils.Encrypt(mysqlPassword, s.encKey)
			if encErr != nil {
				s.logger.Printf("WARN: failed to encrypt password for instance %s: %v", inst.ID, encErr)
			} else {
				if err := s.instRepo.UpdateConnectionPassword(ctx, inst.ID, enc); err != nil {
					s.logger.Printf("WARN: failed to update password for instance %s: %v", inst.ID, err)
				}
			}
			// Also update username if provided
			if mysqlUser != "" {
				if conn, connErr := s.instRepo.GetConnection(ctx, inst.ID); connErr == nil {
					if conn.Username != mysqlUser {
						conn.Username = mysqlUser
						if uErr := s.instRepo.UpdateConnection(ctx, conn); uErr != nil {
							s.logger.Printf("WARN: failed to update username for instance %s: %v", inst.ID, uErr)
						}
					}
				}
			}
		}

		role := replicaRoleFor(clusterType)
		masterID := primaryID
		slaveIDText := ""
		if i == 0 {
			role = primaryRoleFor(clusterType)
			masterID = ""
			slaveIDText = string(slaveIDs)
		}
		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			InstanceID:          inst.ID,
			RunStatus:           defaultString(inst.Status.RunStatus, "running"),
			HealthStatus:        defaultString(inst.Status.HealthStatus, "healthy"),
			Role:                role,
			ReplicationStatus:   clusterType,
			SecondsBehindMaster: 0,
		}); err != nil {
			return fmt.Errorf("update instance %s status failed: %w", inst.ID, err)
		}
		if err := s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       clusterID,
			MasterID:        masterID,
			SlaveIDs:        slaveIDText,
			ReplicationMode: clusterType,
		}); err != nil {
			return fmt.Errorf("update instance %s topology failed: %w", inst.ID, err)
		}
	}
	return nil
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
	if s.agentClient == nil {
		return fmt.Errorf("agent client not configured")
	}
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
	if target, err := s.instanceConnectionConfig(ctx, req.InstanceID); err == nil {
		for k, v := range target {
			config[k] = v
		}
	}

	result, err := s.agentClient.callAgent(ctx, host.Address, host.Port, "/agent/tasks/role-promote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": req.InstanceID,
		"cluster_id":  req.ClusterID,
		"config":      config,
	})
	if err != nil {
		return err
	}
	return switchAgentTaskError("promote instance", result)
}

func (s *SwitchService) demoteInstance(ctx context.Context, host *AgentHostInfo, clusterType string, req RoleSwitchRequest) error {
	if s.agentClient == nil {
		return fmt.Errorf("agent client not configured")
	}
	config := map[string]interface{}{
		"cluster_id":       req.ClusterID,
		"instance_id":      req.InstanceID,
		"target_role":      req.TargetRole,
		"replication_user": req.ReplicationUser,
		"replication_pass": req.ReplicationPass,
		"force":            req.Force,
		"cluster_type":     clusterType,
	}
	if target, err := s.instanceConnectionConfig(ctx, req.InstanceID); err == nil {
		for k, v := range target {
			config[k] = v
		}
	}

	result, err := s.agentClient.callAgent(ctx, host.Address, host.Port, "/agent/tasks/role-demote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": req.InstanceID,
		"cluster_id":  req.ClusterID,
		"config":      config,
	})
	if err != nil {
		return err
	}
	return switchAgentTaskError("demote instance", result)
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
	if s.agentClient == nil {
		return fmt.Errorf("agent client not configured")
	}
	config := map[string]interface{}{
		"cluster_id":   clusterID,
		"instance_id":  oldMasterID,
		"target_role":  replicaRoleFor(clusterType),
		"force":        true,
		"cluster_type": clusterType,
	}
	if target, err := s.instanceConnectionConfig(ctx, oldMasterID); err == nil {
		for k, v := range target {
			config[k] = v
		}
	}

	result, err := s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/role-demote", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": oldMasterID,
		"cluster_id":  clusterID,
		"config":      config,
	})
	if err != nil {
		return err
	}
	return switchAgentTaskError("demote old master", result)
}

func (s *SwitchService) rebuildReplicaTopology(ctx context.Context, clusterID, newMasterID, oldMasterID, clusterType string) ([]string, error) {
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured")
	}
	allInsts, err := s.listClusterInstances(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	rebuilt := make([]string, 0, len(allInsts))
	for _, inst := range allInsts {
		if inst.ID == newMasterID {
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
		if target, err := s.instanceConnectionConfig(ctx, inst.ID); err == nil {
			for k, v := range target {
				config[k] = v
			}
		}
		if masterTarget, err := s.newMasterConnectionConfig(ctx, newMasterID); err == nil {
			for k, v := range masterTarget {
				config[k] = v
			}
		}

		result, err := s.agentClient.callAgent(ctx, hostInfo.Address, hostInfo.Port, "/agent/tasks/role-replica-rebuild", map[string]interface{}{
			"task_id":     uuid.New().String(),
			"instance_id": inst.ID,
			"cluster_id":  clusterID,
			"config":      config,
		})
		if err == nil && switchAgentTaskError("rebuild replica", result) == nil {
			rebuilt = append(rebuilt, inst.ID)
		}
	}
	return rebuilt, nil
}

func (s *SwitchService) instanceConnectionConfig(ctx context.Context, instanceID string) (map[string]interface{}, error) {
	conn, err := s.instRepo.GetConnection(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	pass, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"target_host": conn.Host,
		"target_port": conn.Port,
		"target_user": conn.Username,
		"target_pass": pass,
	}, nil
}

func (s *SwitchService) newMasterConnectionConfig(ctx context.Context, instanceID string) (map[string]interface{}, error) {
	conn, err := s.instRepo.GetConnection(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	pass, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"new_master_host": conn.Host,
		"new_master_port": conn.Port,
		"new_master_user": conn.Username,
		"new_master_pass": pass,
	}, nil
}

func switchAgentTaskError(action string, result *AgentTaskResult) error {
	if result == nil {
		return fmt.Errorf("%s returned empty result", action)
	}
	status := strings.ToLower(strings.TrimSpace(result.Status))
	if isSuccessfulTaskStatus(status) {
		return nil
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = status
	}
	if message == "" {
		message = "empty status"
	}
	return fmt.Errorf("%s did not complete: %s", action, message)
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

	// Persist to database first; on failure, log and continue with in-memory cache.
	if s.historyRepo != nil {
		if err := s.historyRepo.Create(ctx, record); err != nil {
			s.logger.Printf("WARN: failed to persist switch history cluster=%s instance=%s: %v", r.ClusterID, r.InstanceID, err)
		}
	}
	s.mu.Lock()
	s.history[r.ClusterID] = append(s.history[r.ClusterID], *record)
	s.mu.Unlock()

	s.auditRoleSwitch(ctx, r)
}

func (s *SwitchService) auditClusterSwitch(ctx context.Context, req SwitchClusterRequest, result *SwitchClusterResult) {
	if result == nil {
		return
	}
	instanceIDs := req.InstanceIDs
	if len(instanceIDs) == 0 && req.InstanceID != "" {
		instanceIDs = []string{req.InstanceID}
	}
	resourceID := req.InstanceID
	if resourceID == "" {
		resourceID = result.ClusterID
	}
	details := fmt.Sprintf("cluster_id=%s instance_id=%s instance_ids=%s source_type=%s target_type=%s cluster_name=%s vip=%s status=%s message=%s",
		result.ClusterID, req.InstanceID, strings.Join(instanceIDs, ","), result.SourceType, result.TargetType, req.ClusterName, req.VIP, result.Status, result.Message)
	s.auditSwitch(ctx, "switch_cluster_architecture", "switch", "cluster_switch", resourceID, switchAuditResult(result.Status), switchAuditError(result.Status, result.Message), details)
}

func (s *SwitchService) auditRoleSwitch(ctx context.Context, result *RoleSwitchResult) {
	if result == nil {
		return
	}
	resourceID := result.TaskID
	if resourceID == "" {
		resourceID = result.ClusterID + ":" + result.InstanceID
	}
	details := fmt.Sprintf("cluster_id=%s cluster_type=%s instance_id=%s old_role=%s new_role=%s old_master_id=%s new_master_id=%s rebuilt_replicas=%s status=%s message=%s",
		result.ClusterID, result.ClusterType, result.InstanceID, result.OldRole, result.NewRole, result.OldMasterID, result.NewMasterID, strings.Join(result.RebuiltReplicas, ","), result.Status, result.Message)
	s.auditSwitch(ctx, "switch_role_within_cluster", "switch", "role_switch", resourceID, switchAuditResult(result.Status), switchAuditError(result.Status, result.Message), details)
}

func (s *SwitchService) auditSwitch(ctx context.Context, operation, action, resourceType, resourceID, result, errorMsg, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errorMsg,
	})
}

func switchAuditResult(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "timeout", "cancelled", "canceled":
		return "failed"
	default:
		return "success"
	}
}

func switchAuditError(status, message string) string {
	if switchAuditResult(status) != "failed" {
		return ""
	}
	return message
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

// ListRoleSwitchHistory returns role switch history for a cluster.
// It prefers the persisted repository; falls back to in-memory cache.

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

func primaryRoleFor(clusterType string) string {
	switch clusterType {
	case "ha", "mha":
		return "master"
	case "mgr":
		return "primary"
	case "pxc":
		return "primary"
	default:
		return "primary"
	}
}
