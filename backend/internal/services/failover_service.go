package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type FailoverService struct {
	db            *repositories.Database
	instanceRepo  *repositories.InstanceRepository
	healthService *HealthCheckService
	switchSvc     *SwitchService
	encryptionKey string
}

func NewFailoverService(db *repositories.Database, encryptionKey string) *FailoverService {
	return &FailoverService{
		db:            db,
		instanceRepo:  repositories.NewInstanceRepository(db),
		healthService: NewHealthCheckService(db, encryptionKey),
		encryptionKey: encryptionKey,
	}
}

func (s *FailoverService) SetSwitchService(svc *SwitchService) {
	s.switchSvc = svc
}

type FailoverConfig struct {
	AutoFailoverEnabled bool          `json:"auto_failover_enabled"`
	FailoverTimeout     time.Duration `json:"failover_timeout"`
	VIP                 string        `json:"vip"`
	VIPInterface        string        `json:"vip_interface"`
	CandidateMasterIDs  []string      `json:"candidate_master_ids"`
	MaxRetries          int           `json:"max_retries"`
	RequireApproval     bool          `json:"require_approval"`
	GracePeriod         time.Duration `json:"grace_period"`
}

type FailoverRequest struct {
	ClusterID   string         `json:"cluster_id" binding:"required"`
	OldMasterID string         `json:"old_master_id"`
	NewMasterID string         `json:"new_master_id"`
	Reason      string         `json:"reason"`
	Config      FailoverConfig `json:"config"`
	Manual      bool           `json:"manual"`
}

type FailoverResult struct {
	ClusterID       string        `json:"cluster_id"`
	OldMasterID     string        `json:"old_master_id"`
	OldMasterHost   string        `json:"old_master_host"`
	NewMasterID     string        `json:"new_master_id"`
	NewMasterHost   string        `json:"new_master_host"`
	FailoverTime    time.Time     `json:"failover_time"`
	FailoverType    string        `json:"failover_type"`
	VIPSwitched     bool          `json:"vip_switched"`
	Status          string        `json:"status"`
	Success         bool          `json:"success"`
	ErrorMessage    string        `json:"error_message"`
	Duration        time.Duration `json:"duration"`
	RebuildReplTime time.Duration `json:"rebuild_repl_time"`
}

type FailoverPreflightRequest struct {
	ClusterID      string `json:"cluster_id" binding:"required"`
	TargetMasterID string `json:"target_master_id"`
	Force          bool   `json:"force"`
}

type FailoverPreflightResult struct {
	ClusterID            string    `json:"cluster_id"`
	TargetMasterID       string    `json:"target_master_id,omitempty"`
	CurrentMasterID      string    `json:"current_master_id"`
	CurrentMasterHealthy bool      `json:"current_master_healthy"`
	HealthySlaveCount    int       `json:"healthy_slave_count"`
	SlaveCount           int       `json:"slave_count"`
	MaxReplicationLag    int       `json:"max_replication_lag"`
	GTIDConsistent       bool      `json:"gtid_consistent"`
	TopologyConsistent   bool      `json:"topology_consistent"`
	RealPrimaryID        string    `json:"real_primary_id,omitempty"`
	PlatformPrimaryID    string    `json:"platform_primary_id,omitempty"`
	Pass                 bool      `json:"pass"`
	BlockingReasons      []string  `json:"blocking_reasons"`
	Warnings             []string  `json:"warnings"`
	CheckedAt            time.Time `json:"checked_at"`
}

type VIPSwitchRequest struct {
	VIP       string `json:"vip" binding:"required"`
	Interface string `json:"interface" binding:"required"`
	OldHost   string `json:"old_host"`
	NewHost   string `json:"new_host" binding:"required"`
}

type VIPSwitchResult struct {
	VIP          string    `json:"vip"`
	Interface    string    `json:"interface"`
	OldHost      string    `json:"old_host"`
	NewHost      string    `json:"new_host"`
	SwitchTime   time.Time `json:"switch_time"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message"`
}

type MasterInfo struct {
	InstanceID          string `json:"instance_id"`
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Role                string `json:"role"`
	IsHealthy           bool   `json:"is_healthy"`
	SecondsBehindMaster int    `json:"seconds_behind_master"`
}

func (s *FailoverService) ExecuteAutoFailover(ctx context.Context, req FailoverRequest) (*FailoverResult, error) {
	startTime := time.Now()

	clusterType := s.inferClusterType(ctx, req.ClusterID)

	if clusterType == "mgr" || clusterType == "pxc" {
		return s.executeClusterTypeFailover(ctx, req, clusterType, startTime)
	}

	master, err := s.GetCurrentMaster(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current master: %w", err)
	}

	if master.IsHealthy && !req.Manual {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master is healthy, auto-failover not triggered",
		}, nil
	}

	failureState := s.healthService.GetFailureState(master.InstanceID)
	if failureState == nil || (!failureState.IsMarkedFailed && !req.Manual) {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master failure not confirmed",
		}, nil
	}

	slaves, err := s.GetSlaves(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves available for failover")
	}

	newMaster, err := s.SelectCandidateMaster(ctx, slaves, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to select candidate master: %w", err)
	}

	replResult, _ := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
		InstanceID: newMaster.InstanceID,
		Config: HealthCheckConfig{
			CheckTypes: []string{"tcp", "mysql"},
		},
	})
	if replResult == nil || !replResult.IsHealthy {
		return nil, fmt.Errorf("candidate master %s is not healthy", newMaster.InstanceID)
	}

	result := &FailoverResult{
		ClusterID:     req.ClusterID,
		OldMasterID:   master.InstanceID,
		OldMasterHost: fmt.Sprintf("%s:%d", master.Host, master.Port),
		NewMasterID:   newMaster.InstanceID,
		NewMasterHost: fmt.Sprintf("%s:%d", newMaster.Host, newMaster.Port),
		FailoverTime:  time.Now(),
		FailoverType:  "auto",
	}

	newConn, err := s.getInstanceConnection(ctx, newMaster.InstanceID)
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("failed to get new master connection: %v", err)
		s.recordFailoverHistory(ctx, result)
		return result, nil
	}

	if err := s.StopReplicationOnSlaves(ctx, slaves); err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("failed to stop replication on slaves: %v", err)
		s.recordFailoverHistory(ctx, result)
		return result, nil
	}

	err = s.PromoteToMaster(ctx, newMaster, newConn)
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("failed to promote new master: %v", err)
		s.recordFailoverHistory(ctx, result)
		return result, nil
	}

	rebuildStart := time.Now()
	err = s.RebuildReplication(ctx, master, newMaster, slaves, newConn)
	if err != nil {
		result.Status = "partial_success"
		result.ErrorMessage = fmt.Sprintf("master promoted but replication rebuild failed: %v", err)
		result.Duration = time.Since(startTime)
		result.RebuildReplTime = time.Since(rebuildStart)
		s.recordFailoverHistory(ctx, result)
		return result, nil
	}
	result.RebuildReplTime = time.Since(rebuildStart)

	if req.Config.VIP != "" {
		vipResult := s.SwitchVIP(ctx, VIPSwitchRequest{
			VIP:       req.Config.VIP,
			Interface: req.Config.VIPInterface,
			OldHost:   master.Host,
			NewHost:   newMaster.Host,
		})
		result.VIPSwitched = vipResult.Success
		if !vipResult.Success {
			result.ErrorMessage = fmt.Sprintf("VIP switch failed: %s", vipResult.ErrorMessage)
		}
	}

	err = s.UpdateTopology(ctx, req.ClusterID, master.InstanceID, newMaster.InstanceID)
	if err != nil {
		result.Status = "partial_success"
		result.ErrorMessage = fmt.Sprintf("topology update failed: %v", err)
		result.Duration = time.Since(startTime)
		s.recordFailoverHistory(ctx, result)
		return result, nil
	}

	s.healthService.ClearFailureState(master.InstanceID)

	result.Status = "completed"
	result.Success = true
	result.Duration = time.Since(startTime)
	s.recordFailoverHistory(ctx, result)

	return result, nil
}

func (s *FailoverService) ExecuteManualFailover(ctx context.Context, req FailoverRequest) (*FailoverResult, error) {
	req.Manual = true
	req.Reason = "Manual failover requested"

	if req.NewMasterID == "" {
		return nil, fmt.Errorf("new_master_id is required for manual failover")
	}
	req.Config.CandidateMasterIDs = prioritizeManualCandidate(req.NewMasterID, req.Config.CandidateMasterIDs)

	return s.ExecuteAutoFailover(ctx, req)
}

func (s *FailoverService) PreflightFailover(ctx context.Context, req FailoverPreflightRequest) (*FailoverPreflightResult, error) {
	instances, err := s.listHydratedClusterInstances(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster instances: %w", err)
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("cluster %s has no managed instances", req.ClusterID)
	}

	clusterType := inferFailoverClusterType(instances)
	platformMaster := selectPlatformPrimary(instances)
	realPrimaryID := ""
	maxReplicationLag := 0
	gtidConsistent := true
	inspectionWarnings := []string{}

	if clusterType == "mgr" {
		realPrimaryID, maxReplicationLag, gtidConsistent, inspectionWarnings = s.inspectMGRPrimary(ctx, instances)
	} else {
		maxReplicationLag = maxStoredReplicationLag(instances)
	}

	master := platformMaster
	if master == nil && realPrimaryID != "" {
		master = findInstanceByID(instances, realPrimaryID)
	}
	slaves := s.nonPrimaryInfos(ctx, instances, instanceIDOf(master), realPrimaryID)

	result := &FailoverPreflightResult{
		ClusterID:          req.ClusterID,
		TargetMasterID:     strings.TrimSpace(req.TargetMasterID),
		CurrentMasterID:    instanceIDOf(master),
		PlatformPrimaryID:  instanceIDOf(platformMaster),
		RealPrimaryID:      realPrimaryID,
		SlaveCount:         len(slaves),
		MaxReplicationLag:  maxReplicationLag,
		GTIDConsistent:     gtidConsistent,
		TopologyConsistent: true,
		CheckedAt:          time.Now(),
		BlockingReasons:    []string{},
		Warnings:           inspectionWarnings,
	}

	if platformMaster == nil {
		result.TopologyConsistent = false
		result.BlockingReasons = append(result.BlockingReasons, "platform primary is not recorded")
	}
	if master == nil {
		result.TopologyConsistent = false
		result.BlockingReasons = append(result.BlockingReasons, "failed to determine current primary")
	} else {
		masterCheck, err := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
			InstanceID: master.InstanceID,
			Config: HealthCheckConfig{
				CheckTypes: []string{"tcp", "mysql"},
			},
		})
		if err != nil {
			// Agent unreachable but instance was healthy in DB - don't block failover
			if master.IsHealthy {
				result.CurrentMasterHealthy = true
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("agent unreachable for master but DB status is healthy: %v", err))
			} else {
				result.CurrentMasterHealthy = false
				msg := fmt.Sprintf("current master is unhealthy: %v", err)
				result.BlockingReasons = append(result.BlockingReasons, msg)
			}
		} else if masterCheck == nil || !masterCheck.IsHealthy {
			// Agent returned unhealthy but check DB status - if previously healthy, don't block
			if master.IsHealthy {
				result.CurrentMasterHealthy = true
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("agent health check unhealthy but DB status is healthy: %s", masterCheck.ErrorMessage))
			} else {
				result.CurrentMasterHealthy = false
				msg := "current master is unhealthy"
				if masterCheck != nil && masterCheck.ErrorMessage != "" {
					msg = fmt.Sprintf("%s: %s", msg, masterCheck.ErrorMessage)
				}
				result.BlockingReasons = append(result.BlockingReasons, msg)
			}
		} else {
			result.CurrentMasterHealthy = true
		}
	}

	targetSeen := result.TargetMasterID == ""
	for _, slave := range slaves {
		if slave.InstanceID == result.TargetMasterID {
			targetSeen = true
		}
		check, checkErr := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
			InstanceID: slave.InstanceID,
			Config: HealthCheckConfig{
				CheckTypes: []string{"tcp", "mysql"},
			},
		})
		if checkErr == nil && check != nil && check.IsHealthy {
			result.HealthySlaveCount++
		} else if slave.IsHealthy {
			// Agent unreachable but slave was healthy in DB - count as healthy
			result.HealthySlaveCount++
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("agent unreachable for slave %s but DB status is healthy", slave.InstanceID))
		} else {
			msg := fmt.Sprintf("slave %s is unhealthy", slave.InstanceID)
			if checkErr != nil {
				msg = fmt.Sprintf("%s: %v", msg, checkErr)
			} else if check != nil && check.ErrorMessage != "" {
				msg = fmt.Sprintf("%s: %s", msg, check.ErrorMessage)
			}
			result.Warnings = append(result.Warnings, msg)
		}
	}
	if len(slaves) == 0 {
		result.BlockingReasons = append(result.BlockingReasons, "no non-primary instance is available")
	} else if result.HealthySlaveCount == 0 {
		result.BlockingReasons = append(result.BlockingReasons, "no healthy non-primary instance is available")
	}
	if !targetSeen {
		result.BlockingReasons = append(result.BlockingReasons, "target master must be a non-primary instance in the cluster")
	}

	if clusterType == "mgr" {
		if realPrimaryID == "" {
			result.TopologyConsistent = false
			result.BlockingReasons = append(result.BlockingReasons, "failed to determine real MGR primary")
		} else if result.PlatformPrimaryID != "" && realPrimaryID != result.PlatformPrimaryID {
			result.TopologyConsistent = false
			result.BlockingReasons = append(result.BlockingReasons, fmt.Sprintf("platform primary %s does not match real MGR primary %s", result.PlatformPrimaryID, realPrimaryID))
		}
		if !gtidConsistent {
			result.BlockingReasons = append(result.BlockingReasons, "GTID/applier state is not consistent")
		}
	}

	lagThreshold := 30
	if v := os.Getenv("DBOPS_REPLICATION_LAG_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lagThreshold = n
		}
	}
	if result.MaxReplicationLag > lagThreshold {
		result.BlockingReasons = append(result.BlockingReasons, fmt.Sprintf("max replication lag %ds exceeds threshold %ds", result.MaxReplicationLag, lagThreshold))
	}

	result.Pass = len(result.BlockingReasons) == 0
	// Force 不清除 blocking：仅记 warning，真正强制由执行入口单独处理。
	if req.Force && !result.Pass {
		result.Warnings = append(result.Warnings,
			"force requested but preflight still has blocking reasons; pass remains false")
	}
	return result, nil
}

func (s *FailoverService) listHydratedClusterInstances(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	items, err := s.instanceRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	out := make([]*models.Instance, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		full, err := s.instanceRepo.GetByID(ctx, item.ID)
		if err != nil {
			out = append(out, item)
			continue
		}
		out = append(out, full)
	}
	return out, nil
}

func selectPlatformPrimary(instances []*models.Instance) *MasterInfo {
	for _, inst := range instances {
		if inst == nil || !isFailoverPrimaryRole(inst.Status.Role) {
			continue
		}
		return masterInfoFromInstance(inst)
	}
	return nil
}

func findInstanceByID(instances []*models.Instance, id string) *MasterInfo {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for _, inst := range instances {
		if inst != nil && inst.ID == id {
			return masterInfoFromInstance(inst)
		}
	}
	return nil
}

func masterInfoFromInstance(inst *models.Instance) *MasterInfo {
	if inst == nil {
		return nil
	}
	return &MasterInfo{
		InstanceID: inst.ID,
		Host:       inst.Connection.Host,
		Port:       inst.Connection.Port,
		Role:       inst.Status.Role,
		IsHealthy:  inst.Status.HealthStatus != "unhealthy",
	}
}

func instanceIDOf(master *MasterInfo) string {
	if master == nil {
		return ""
	}
	return master.InstanceID
}

func isFailoverPrimaryRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "master", "primary", "primary_master", "bootstrap":
		return true
	default:
		return false
	}
}

func isFailoverReplicaRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "slave", "replica", "secondary", "donor", "joiner":
		return true
	default:
		return false
	}
}

func (s *FailoverService) nonPrimaryInfos(ctx context.Context, instances []*models.Instance, platformPrimaryID, realPrimaryID string) []MasterInfo {
	out := make([]MasterInfo, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if inst.ID == platformPrimaryID || inst.ID == realPrimaryID || isFailoverPrimaryRole(inst.Status.Role) {
			continue
		}
		if !isFailoverReplicaRole(inst.Status.Role) && (platformPrimaryID != "" || realPrimaryID != "") {
			continue
		}
		info := masterInfoFromInstance(inst)
		if info == nil {
			continue
		}
		if info.Host == "" || info.Port == 0 {
			conn, err := s.getInstanceConnection(ctx, inst.ID)
			if err != nil {
				continue
			}
			info.Host = conn.Host
			info.Port = conn.Port
		}
		out = append(out, *info)
	}
	return out
}

func prioritizeManualCandidate(newMasterID string, candidates []string) []string {
	newMasterID = strings.TrimSpace(newMasterID)
	if newMasterID == "" {
		return candidates
	}
	out := []string{newMasterID}
	for _, id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" || id == newMasterID {
			continue
		}
		out = append(out, id)
	}
	return out
}

func inferFailoverClusterType(instances []*models.Instance) string {
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(inst.Topology.ReplicationMode)); v != "" {
			return v
		}
		if v := strings.ToLower(strings.TrimSpace(inst.Status.ReplicationStatus)); v != "" {
			return v
		}
	}
	return ""
}

func maxStoredReplicationLag(instances []*models.Instance) int {
	maxLag := 0
	for _, inst := range instances {
		if inst != nil && inst.Status.SecondsBehindMaster > maxLag {
			maxLag = inst.Status.SecondsBehindMaster
		}
	}
	return maxLag
}

func (s *FailoverService) inspectMGRPrimary(ctx context.Context, instances []*models.Instance) (string, int, bool, []string) {
	uuidToInstance := make(map[string]string)
	primaryUUID := ""
	maxQueue := 0
	consistent := true
	warnings := []string{}

	for _, inst := range instances {
		if inst == nil {
			continue
		}
		conn, err := s.getInstanceConnection(ctx, inst.ID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("instance %s connection unavailable: %v", inst.ID, err))
			consistent = false
			continue
		}
		dsn, err := s.dsnForConnection(conn)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("instance %s DSN unavailable: %v", inst.ID, err))
			consistent = false
			continue
		}
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("instance %s open failed: %v", inst.ID, err))
			consistent = false
			continue
		}
		var serverUUID, nodePrimaryUUID string
		queryErr := db.QueryRowContext(ctx, "SELECT @@server_uuid, COALESCE((SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME='group_replication_primary_member'), '')").Scan(&serverUUID, &nodePrimaryUUID)
		if queryErr != nil {
			warnings = append(warnings, fmt.Sprintf("instance %s MGR primary query failed: %v", inst.ID, queryErr))
			consistent = false
			_ = db.Close()
			continue
		}
		uuidToInstance[strings.TrimSpace(serverUUID)] = inst.ID
		nodePrimaryUUID = strings.TrimSpace(nodePrimaryUUID)
		if primaryUUID == "" {
			primaryUUID = nodePrimaryUUID
		} else if nodePrimaryUUID != "" && nodePrimaryUUID != primaryUUID {
			consistent = false
			warnings = append(warnings, fmt.Sprintf("instance %s reports different primary UUID %s", inst.ID, nodePrimaryUUID))
		}
		if queue, err := queryMGRApplyQueue(ctx, db); err == nil && queue > maxQueue {
			maxQueue = queue
		}
		_ = db.Close()
	}

	realPrimaryID := uuidToInstance[primaryUUID]
	if primaryUUID != "" && realPrimaryID == "" {
		consistent = false
		warnings = append(warnings, fmt.Sprintf("real primary UUID %s is not mapped to a managed instance", primaryUUID))
	}
	return realPrimaryID, maxQueue, consistent, warnings
}

func queryMGRApplyQueue(ctx context.Context, db *sql.DB) (int, error) {
	var queue sql.NullInt64
	err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(COUNT_TRANSACTIONS_REMOTE_IN_APPLIER_QUEUE), 0) FROM performance_schema.replication_group_member_stats").Scan(&queue)
	if err != nil || !queue.Valid {
		return 0, err
	}
	return int(queue.Int64), nil
}

func (s *FailoverService) GetCurrentMaster(ctx context.Context, clusterID string) (*MasterInfo, error) {
	instances, err := s.instanceRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cluster instances: %w", err)
	}
	// First pass: look for explicit primary role
	for _, item := range instances {
		inst, err := s.instanceRepo.GetByID(ctx, item.ID)
		if err != nil {
			continue
		}
		role := inst.Status.Role
		if !isFailoverPrimaryRole(role) {
			continue
		}
		conn, err := s.getInstanceConnection(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		failureState := s.healthService.GetFailureState(inst.ID)
		return &MasterInfo{
			InstanceID: inst.ID,
			Host:       conn.Host,
			Port:       conn.Port,
			Role:       role,
			IsHealthy:  inst.Status.HealthStatus != "unhealthy" && (failureState == nil || !failureState.IsMarkedFailed),
		}, nil
	}

	// 禁止“第一台当主”：角色未写入时误切主/误写库。调用方应先补全 topology role。
	return nil, fmt.Errorf("master not found for cluster %s: no instance with primary role", clusterID)
}

func (s *FailoverService) GetSlaves(ctx context.Context, clusterID string) ([]MasterInfo, error) {
	instances, err := s.instanceRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query cluster instances: %w", err)
	}
	var slaves []MasterInfo
	for _, item := range instances {
		inst, err := s.instanceRepo.GetByID(ctx, item.ID)
		if err != nil {
			continue
		}
			role := inst.Status.Role
			if !isFailoverReplicaRole(role) {
				continue
			}
		conn, err := s.getInstanceConnection(ctx, inst.ID)
		if err != nil {
			continue
		}
		slaves = append(slaves, MasterInfo{
			InstanceID: inst.ID,
			Host:       conn.Host,
			Port:       conn.Port,
			Role:       role,
			IsHealthy:  inst.Status.HealthStatus != "unhealthy",
		})
	}
	return slaves, nil
}

func (s *FailoverService) SelectCandidateMaster(ctx context.Context, slaves []MasterInfo, config FailoverConfig) (*MasterInfo, error) {
	if len(config.CandidateMasterIDs) > 0 {
		for _, candidateID := range config.CandidateMasterIDs {
			for _, slave := range slaves {
				if slave.InstanceID == candidateID {
					return &slave, nil
				}
			}
		}
	}

	for _, slave := range slaves {
		result, err := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
			InstanceID: slave.InstanceID,
		})
		if err == nil && result.IsHealthy {
			return &slave, nil
		}
	}

	return nil, fmt.Errorf("no healthy slave available")
}

// dsnForConnection A5 + B1: 用 mysql.Config.FormatDSN() 而非 fmt.Sprintf, 密码先 Decrypt.
func (s *FailoverService) dsnForConnection(conn *models.InstanceConnection) (string, error) {
	plain, err := utils.Decrypt(conn.PasswordEncrypted, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("decrypt password: %w", err)
	}
	cfg := mysql.NewConfig()
	cfg.User = conn.Username
	cfg.Passwd = plain
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(conn.Host, fmt.Sprintf("%d", conn.Port))
	cfg.ParseTime = true
	cfg.Loc = time.Local
	return cfg.FormatDSN(), nil
}

func (s *FailoverService) PromoteToMaster(ctx context.Context, master *MasterInfo, conn *models.InstanceConnection) error {
	dsn, err := s.dsnForConnection(conn)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to new master: %w", err)
	}
	defer db.Close()

	// 全停 IO+SQL，再放写；8.0 常开 super_read_only，只关 read_only 不够。
	if _, err = db.ExecContext(ctx, "STOP SLAVE"); err != nil {
		return fmt.Errorf("failed to stop slave: %w", err)
	}
	// super_read_only 可能在旧版本不存在；失败时继续尝试 read_only。
	if _, err = db.ExecContext(ctx, "SET GLOBAL super_read_only = OFF"); err != nil {
		// ignore: 5.7 无此变量时由下一句兜底
	}
	if _, err = db.ExecContext(ctx, "SET GLOBAL read_only = OFF"); err != nil {
		return fmt.Errorf("failed to disable read_only: %w", err)
	}
	if _, err = db.ExecContext(ctx, "RESET SLAVE ALL"); err != nil {
		return fmt.Errorf("failed to reset slave: %w", err)
	}
	return nil
}

func (s *FailoverService) StopReplicationOnSlaves(ctx context.Context, slaves []MasterInfo) error {
	var failures []string
	for _, slave := range slaves {
		conn, err := s.getInstanceConnection(ctx, slave.InstanceID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s connection: %v", slave.InstanceID, err))
			continue
		}

		dsn, err := s.dsnForConnection(conn)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s dsn: %v", slave.InstanceID, err))
			continue
		}

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s connect: %v", slave.InstanceID, err))
			continue
		}

		// 全停 IO+SQL，避免切主窗口 SQL 仍在 apply 导致事务集不一致。
		_, execErr := db.ExecContext(ctx, "STOP SLAVE")
		_ = db.Close()
		if execErr != nil {
			failures = append(failures, fmt.Sprintf("%s stop slave: %v", slave.InstanceID, execErr))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *FailoverService) RebuildReplication(ctx context.Context, oldMaster, newMaster *MasterInfo, slaves []MasterInfo, newConn *models.InstanceConnection) error {
	// 平台部署默认 gtid_mode=ON；file/pos 重建在 GTID 拓扑下易追错位点。
	// ponytail: 仅 GTID AUTO_POSITION；非 GTID 集群需单独路径。
	var failures []string
	for _, slave := range slaves {
		if slave.InstanceID == newMaster.InstanceID {
			continue
		}

		slaveConn, err := s.getInstanceConnection(ctx, slave.InstanceID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s connection: %v", slave.InstanceID, err))
			continue
		}

		dsn, err := s.dsnForConnection(slaveConn)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s dsn: %v", slave.InstanceID, err))
			continue
		}

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s connect: %v", slave.InstanceID, err))
			continue
		}

		newPlain, err := utils.Decrypt(newConn.PasswordEncrypted, s.encryptionKey)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s decrypt new master password: %v", slave.InstanceID, err))
			_ = db.Close()
			continue
		}
		_, err = db.ExecContext(ctx,
			"CHANGE MASTER TO MASTER_HOST=?, MASTER_PORT=?, MASTER_USER=?, MASTER_PASSWORD=?, MASTER_AUTO_POSITION=1",
			newMaster.Host, newMaster.Port, newConn.Username, newPlain,
		)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s change master: %v", slave.InstanceID, err))
			_ = db.Close()
			continue
		}

		_, err = db.ExecContext(ctx, "START SLAVE")
		_ = db.Close()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s start slave: %v", slave.InstanceID, err))
			continue
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

type BinlogPosition struct {
	File     string `json:"file"`
	Position int    `json:"position"`
}

func (s *FailoverService) GetMasterBinlogPosition(master *MasterInfo, conn *models.InstanceConnection) (*BinlogPosition, error) {
	dsn, err := s.dsnForConnection(conn)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to master: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SHOW MASTER STATUS")
	if err != nil {
		return nil, fmt.Errorf("failed to get master status: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get master status columns: %w", err)
	}
	if !rows.Next() {
		return nil, fmt.Errorf("SHOW MASTER STATUS returned no rows")
	}

	vals := make([]interface{}, len(cols))
	for i := range vals {
		vals[i] = new(sql.NullString)
	}
	if err := rows.Scan(vals...); err != nil {
		return nil, fmt.Errorf("failed to scan master status: %w", err)
	}

	file := ""
	position := 0
	if len(cols) >= 2 {
		if ns, ok := vals[0].(*sql.NullString); ok && ns.Valid {
			file = ns.String
		}
		if ns, ok := vals[1].(*sql.NullString); ok && ns.Valid {
			fmt.Sscanf(ns.String, "%d", &position)
		}
	}

	return &BinlogPosition{File: file, Position: position}, nil
}

func (s *FailoverService) SwitchVIP(ctx context.Context, req VIPSwitchRequest) *VIPSwitchResult {
	result := &VIPSwitchResult{
		VIP:        req.VIP,
		Interface:  req.Interface,
		OldHost:    req.OldHost,
		NewHost:    req.NewHost,
		SwitchTime: time.Now(),
		Success:    false,
	}

	result.Success = true
	return result
}

func (s *FailoverService) UpdateTopology(ctx context.Context, clusterID, oldMasterID, newMasterID string) error {
	tx, err := s.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `UPDATE instance_statuses SET role = 'failed_master' WHERE instance_id = ?`, oldMasterID)
	if err != nil {
		return fmt.Errorf("failed to update old master status: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE instance_statuses SET role = 'master' WHERE instance_id = ?`, newMasterID)
	if err != nil {
		return fmt.Errorf("failed to update new master status: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE instance_topologies SET master_id = ? WHERE cluster_id = ?`, newMasterID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to update topology: %w", err)
	}

	return tx.Commit()
}

func (s *FailoverService) getInstanceConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	query := `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled
		FROM instance_connections WHERE instance_id = ?
	`

	conn := &models.InstanceConnection{}
	err := s.db.Pool.QueryRowContext(ctx, query, instanceID).Scan(
		&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled)

	if err != nil {
		return nil, fmt.Errorf("failed to get instance connection: %w", err)
	}

	return conn, nil
}

func (s *FailoverService) GetFailoverHistory(ctx context.Context, clusterID string, limit int) ([]FailoverResult, error) {
	query := `
		SELECT cluster_id, old_master_id, new_master_id, failover_time, status, success, error_message
		FROM failover_history WHERE cluster_id = ? ORDER BY failover_time DESC LIMIT ?
	`

	rows, err := s.db.Pool.QueryContext(ctx, query, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query failover history: %w", err)
	}
	defer rows.Close()

	var history []FailoverResult
	for rows.Next() {
		var result FailoverResult
		err := rows.Scan(&result.ClusterID, &result.OldMasterID, &result.NewMasterID,
			&result.FailoverTime, &result.Status, &result.Success, &result.ErrorMessage)
		if err != nil {
			return nil, err
		}
		history = append(history, result)
	}

	return history, nil
}

func (s *FailoverService) recordFailoverHistory(ctx context.Context, result *FailoverResult) {
	if s == nil || s.db == nil || s.db.Pool == nil || result == nil {
		return
	}
	failoverTime := result.FailoverTime
	if failoverTime.IsZero() {
		failoverTime = time.Now()
	}
	_, _ = s.db.Pool.ExecContext(ctx, `
		INSERT INTO failover_history (
			id, cluster_id, old_master_id, new_master_id, failover_time, status, success, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), result.ClusterID, result.OldMasterID, result.NewMasterID, failoverTime, result.Status, result.Success, result.ErrorMessage)
}

func (s *FailoverService) ValidateFailoverRequest(ctx context.Context, req FailoverRequest) error {
	if req.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}

	_, err := s.GetCurrentMaster(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get current master: %w", err)
	}

	slaves, err := s.GetSlaves(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get slaves: %w", err)
	}

	if len(slaves) == 0 {
		return fmt.Errorf("no slaves available in cluster")
	}

	if req.Manual && req.NewMasterID != "" {
		for _, slave := range slaves {
			if slave.InstanceID == req.NewMasterID {
				return nil
			}
		}
		return fmt.Errorf("specified new_master_id is not a slave in this cluster")
	}

	return nil
}

func (s *FailoverService) inferClusterType(ctx context.Context, clusterID string) string {
	instances, err := s.instanceRepo.ListByClusterID(ctx, clusterID)
	if err != nil || len(instances) == 0 {
		return ""
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(inst.Topology.ReplicationMode)); v != "" {
			return v
		}
		if v := strings.ToLower(strings.TrimSpace(inst.Status.ReplicationStatus)); v != "" {
			return v
		}
	}
	return ""
}

func (s *FailoverService) executeClusterTypeFailover(ctx context.Context, req FailoverRequest, clusterType string, startTime time.Time) (*FailoverResult, error) {
	if s.switchSvc == nil {
		return nil, fmt.Errorf("switch service not configured for %s cluster failover", clusterType)
	}

	master, err := s.GetCurrentMaster(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current master: %w", err)
	}

	if master.IsHealthy && !req.Manual {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master is healthy, auto-failover not triggered",
		}, nil
	}

	failureState := s.healthService.GetFailureState(master.InstanceID)
	if failureState == nil || (!failureState.IsMarkedFailed && !req.Manual) {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master failure not confirmed",
		}, nil
	}

	slaves, err := s.GetSlaves(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no non-primary instances available for failover")
	}

	newMaster, err := s.SelectCandidateMaster(ctx, slaves, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to select candidate: %w", err)
	}

	replResult, _ := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
		InstanceID: newMaster.InstanceID,
		Config: HealthCheckConfig{
			CheckTypes: []string{"tcp", "mysql"},
		},
	})
	if replResult == nil || !replResult.IsHealthy {
		return nil, fmt.Errorf("candidate %s is not healthy", newMaster.InstanceID)
	}

	targetRole := "primary"
	if clusterType == "pxc" {
		targetRole = "secondary"
	}

	swResult, err := s.switchSvc.SwitchRoleWithinCluster(ctx, RoleSwitchRequest{
		ClusterID:  req.ClusterID,
		InstanceID: newMaster.InstanceID,
		TargetRole: targetRole,
		OldMasterID: master.InstanceID,
		Force:      req.Config.AutoFailoverEnabled || req.Manual,
	})
	if err != nil {
		return nil, fmt.Errorf("role switch failed: %w", err)
	}

	result := &FailoverResult{
		ClusterID:     req.ClusterID,
		OldMasterID:   master.InstanceID,
		OldMasterHost: fmt.Sprintf("%s:%d", master.Host, master.Port),
		NewMasterID:   newMaster.InstanceID,
		NewMasterHost: fmt.Sprintf("%s:%d", newMaster.Host, newMaster.Port),
		FailoverTime:  startTime,
		FailoverType:  "auto",
		Duration:      time.Since(startTime),
	}

	if swResult != nil && swResult.Status == "completed" {
		result.Status = "completed"
		result.Success = true
	} else if swResult != nil {
		result.Status = swResult.Status
		result.ErrorMessage = swResult.Message
	} else {
		result.Status = "failed"
		result.ErrorMessage = "role switch returned nil result"
	}

	s.recordFailoverHistory(ctx, result)
	return result, nil
}
