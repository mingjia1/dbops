package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

// archTypeToPluginName maps a cluster architecture type to its plugin name.
// The plugin executor uses these names to dispatch to the correct join/leave handler.
func archTypeToPluginName(archType string) string {
	switch strings.ToLower(strings.TrimSpace(archType)) {
	case "ha":
		return "replica-addon"
	case "mha":
		return "mha-addon"
	case "mgr":
		return "mgr-addon"
	case "pxc":
		return "pxc-galera-addon"
	default:
		return archType
	}
}

type ScaleService struct {
	orchestrator *DeployOrchestrator
	instRepo     InstanceRepositoryInterface
	pluginExec   *plugins.Executor
	auditSvc     *AuditService
}

func NewScaleService(orchestrator *DeployOrchestrator, instRepo InstanceRepositoryInterface, pluginExec *plugins.Executor, auditSvc ...*AuditService) *ScaleService {
	s := &ScaleService{
		orchestrator: orchestrator,
		instRepo:     instRepo,
		pluginExec:   pluginExec,
	}
	if len(auditSvc) > 0 && auditSvc[0] != nil {
		s.auditSvc = auditSvc[0]
	}
	return s
}

type ScaleOutRequest struct {
	ClusterID   string
	Flavor      string
	ArchType    string
	EncKey      string
	NewNodes    []OrchestratorNode
	ExistingEnv plugins.PluginEnv
}

type ScaleOutResult struct {
	ClusterID   string   `json:"cluster_id"`
	NewInstIDs  []string `json:"new_instance_ids"`
	Status      string   `json:"status"`
	Message     string   `json:"message"`
}

func (s *ScaleService) ScaleOut(ctx context.Context, req ScaleOutRequest) (*ScaleOutResult, error) {
	if len(req.NewNodes) == 0 {
		return nil, fmt.Errorf("at least one new node is required")
	}

	creds, err := s.orchestrator.ensureCredentials(ctx, DeployOrchestratorRequest{
		ClusterID: req.ClusterID,
		EncKey:    req.EncKey,
		Flavor:    req.Flavor,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure credentials: %w", err)
	}

	kernelPluginName := req.Flavor + "-core"
	var instIDs []string
	roleForNewNode := scaleOutReplicaRole(req.ArchType)

	// H2: Fetch existing cluster instances so that arch plugins (e.g. MGR)
	// can derive per-node local_port from the total node count.
	var existingNodes []plugins.PluginNode
	if s.instRepo != nil {
		if existingInstances, listErr := s.instRepo.ListByClusterID(ctx, req.ClusterID); listErr == nil {
			for _, inst := range existingInstances {
				if inst == nil {
					continue
				}
				pn := plugins.PluginNode{HostID: inst.ID}
				if inst.HostID != nil {
					pn.HostID = *inst.HostID
				}
				if conn, connErr := s.instRepo.GetConnection(ctx, inst.ID); connErr == nil && conn != nil {
					pn.Address = conn.Host
					pn.MySQLPort = conn.Port
					pn.DataDir = conn.Datadir
					pn.Basedir = conn.Basedir
				}
				existingNodes = append(existingNodes, pn)
			}
		}
	}

	for _, node := range req.NewNodes {
		env := plugins.PluginEnv{
			ClusterID: req.ClusterID,
			Nodes: []plugins.PluginNode{
				{
					HostID:    node.HostID,
					Address:   node.Address,
					AgentPort: node.AgentPort,
					MySQLPort: node.MySQLPort,
					Role:      roleForNewNode,
					DataDir:   node.DataDir,
					Basedir:   node.Basedir,
				},
			},
			Credentials: *creds,
		}

		if _, err := s.pluginExec.RunExecute(ctx, kernelPluginName, env, nil); err != nil {
			return &ScaleOutResult{
				ClusterID: req.ClusterID,
				Status:    "failed",
				Message:   fmt.Sprintf("deploy replica on %s: %v", node.Address, err),
			}, err
		}

		archPluginName := archTypeToPluginName(req.ArchType)
		// H2: Include existing nodes in joinEnv so plugins can derive
		// unique per-node local_port from len(Nodes).
		newPluginNode := plugins.PluginNode{
			HostID:    node.HostID,
			Address:   node.Address,
			AgentPort: node.AgentPort,
			MySQLPort: node.MySQLPort,
			Role:      roleForNewNode,
			DataDir:   node.DataDir,
			Basedir:   node.Basedir,
		}
		joinNodes := make([]plugins.PluginNode, 0, len(existingNodes)+1)
		joinNodes = append(joinNodes, existingNodes...)
		joinNodes = append(joinNodes, newPluginNode)
		joinEnv := plugins.PluginEnv{
			ClusterID:   req.ClusterID,
			Nodes:       joinNodes,
			Credentials: *creds,
		}

		if err := s.pluginExec.RunJoin(ctx, archPluginName, joinEnv, plugins.PluginNode{
			HostID:    node.HostID,
			Address:   node.Address,
			AgentPort: node.AgentPort,
			MySQLPort: node.MySQLPort,
			Role:      roleForNewNode,
		}); err != nil {
			log.Printf("WARN: arch join failed for %s: %v", node.Address, err)
		}

		// H2: Track newly added node so subsequent iterations see it
		// in existingNodes and get a unique local_port.
		existingNodes = append(existingNodes, newPluginNode)

		instID := uuid.New().String()
		if s.instRepo != nil {
			inst := &models.Instance{
				ID:        instID,
				Name:      fmt.Sprintf("%s-%s", roleForNewNode, node.Address),
				ClusterID: req.ClusterID,
			}
			if node.HostID != "" {
				inst.HostID = &node.HostID
			}
			if err := s.instRepo.Create(ctx, inst); err != nil {
				log.Printf("WARN: failed to persist instance: %v", err)
			} else {
				_ = s.instRepo.CreateConnection(ctx, &models.InstanceConnection{
					InstanceID: instID,
					Host:       node.Address,
					Port:       node.MySQLPort,
					Username:   "root",
					Basedir:    node.Basedir,
					Datadir:    node.DataDir,
				})
				_ = s.instRepo.UpsertStatus(ctx, instID, &models.InstanceStatus{
					InstanceID:        instID,
					RunStatus:         "running",
					HealthStatus:      "healthy",
					Role:              roleForNewNode,
					ReplicationStatus: req.ArchType,
				})
			}
		}
		instIDs = append(instIDs, instID)
	}
	s.refreshClusterTopology(ctx, req.ClusterID, req.ArchType)

	// L3: Record audit log for scale-out operation
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Operation:    "scale_out",
			ResourceType: "cluster",
			ResourceID:   req.ClusterID,
			Action:       fmt.Sprintf("add %d node(s)", len(req.NewNodes)),
			Details:      fmt.Sprintf("arch=%s nodes=%d", req.ArchType, len(req.NewNodes)),
			Result:       "success",
		})
	}

	return &ScaleOutResult{
		ClusterID:  req.ClusterID,
		NewInstIDs: instIDs,
		Status:     "completed",
		Message:    fmt.Sprintf("%d new nodes added to cluster", len(req.NewNodes)),
	}, nil
}

func scaleOutReplicaRole(archType string) string {
	switch strings.ToLower(strings.TrimSpace(archType)) {
	case "mgr", "pxc":
		return "secondary"
	default:
		return "replica"
	}
}

func scaleOutPrimaryRoles(archType string) []string {
	switch strings.ToLower(strings.TrimSpace(archType)) {
	case "mgr":
		return []string{"primary"}
	case "pxc":
		return []string{"primary", "bootstrap"}
	default:
		return []string{"master", "primary"}
	}
}

func (s *ScaleService) refreshClusterTopology(ctx context.Context, clusterID, archType string) {
	if s.instRepo == nil {
		return
	}
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil || len(instances) == 0 {
		return
	}
	primaryRoles := scaleOutPrimaryRoles(archType)
	primaryID := ""
	replicaIDs := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(inst.Status.Role))
		for _, primaryRole := range primaryRoles {
			if role == primaryRole {
				primaryID = inst.ID
				break
			}
		}
	}
	if primaryID == "" && len(instances) > 0 && instances[0] != nil {
		primaryID = instances[0].ID
	}
	for _, inst := range instances {
		if inst == nil || inst.ID == primaryID {
			continue
		}
		replicaIDs = append(replicaIDs, inst.ID)
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if inst.ID == primaryID {
			_ = s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
				InstanceID:      inst.ID,
				ClusterID:       clusterID,
				MasterID:        "",
				SlaveIDs:        mustJSONString(replicaIDs),
				ReplicationMode: archType,
			})
			continue
		}
		_ = s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       clusterID,
			MasterID:        primaryID,
			SlaveIDs:        "",
			ReplicationMode: archType,
		})
	}
}

func mustJSONString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprintf("[\"%s\"]", strings.Join(values, "\",\""))
}

type ScaleInRequest struct {
	ClusterID    string
	RemoveNodeID string
	Flavor       string
	ArchType     string
	IsPrimary    bool
}

type ScaleInResult struct {
	ClusterID   string `json:"cluster_id"`
	RemovedID   string `json:"removed_id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

func (s *ScaleService) ScaleIn(ctx context.Context, req ScaleInRequest) (*ScaleInResult, error) {
	if req.RemoveNodeID == "" {
		return nil, fmt.Errorf("remove node ID is required")
	}

	if req.IsPrimary {
		return nil, fmt.Errorf("cannot remove primary node directly: perform role switch first")
	}

	if req.ArchType != "" && req.ArchType != "single" {
		archPluginName := archTypeToPluginName(req.ArchType)

		var leaveNode plugins.PluginNode
		if s.instRepo != nil {
			if inst, err := s.instRepo.GetByID(ctx, req.RemoveNodeID); err == nil && inst != nil {
				leaveNode = plugins.PluginNode{
					HostID:    inst.ID,
					Address:   inst.Connection.Host,
					MySQLPort: inst.Connection.Port,
				}
				if inst.HostID != nil {
					leaveNode.HostID = *inst.HostID
				}
			}
		}
		if leaveNode.Address == "" {
			leaveNode.Address = req.RemoveNodeID
		}

		leaveEnv := plugins.PluginEnv{
			ClusterID: req.ClusterID,
			Nodes:     []plugins.PluginNode{leaveNode},
		}
		if err := s.pluginExec.RunLeave(ctx, archPluginName, leaveEnv, leaveNode); err != nil {
			log.Printf("WARN: arch leave failed for %s: %v", req.RemoveNodeID, err)
		}
	}

	if s.instRepo != nil {
		if err := s.instRepo.Delete(ctx, req.RemoveNodeID); err != nil {
			return nil, fmt.Errorf("delete instance: %w", err)
		}
	}

	// L3: Record audit log for scale-in operation
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Operation:    "scale_in",
			ResourceType: "cluster",
			ResourceID:   req.ClusterID,
			Action:       fmt.Sprintf("remove node %s", req.RemoveNodeID),
			Details:      fmt.Sprintf("arch=%s removed=%s", req.ArchType, req.RemoveNodeID),
			Result:       "success",
		})
	}

	return &ScaleInResult{
		ClusterID: req.ClusterID,
		RemovedID: req.RemoveNodeID,
		Status:    "completed",
		Message:   fmt.Sprintf("Node %s removed from cluster %s", req.RemoveNodeID, req.ClusterID),
	}, nil
}

type RebuildRequest struct {
	ClusterID  string
	InstanceID string
	Flavor     string
	ArchType   string
	EncKey     string
	Node       OrchestratorNode
}

type RebuildResult struct {
	ClusterID  string `json:"cluster_id"`
	InstanceID string `json:"instance_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DurationMs int64  `json:"duration_ms"`
}

func (s *ScaleService) RebuildNode(ctx context.Context, req RebuildRequest) (*RebuildResult, error) {
	start := time.Now()

	creds, err := s.orchestrator.ensureCredentials(ctx, DeployOrchestratorRequest{
		ClusterID: req.ClusterID,
		EncKey:    req.EncKey,
		Flavor:    req.Flavor,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure credentials: %w", err)
	}

	kernelPluginName := req.Flavor + "-core"
	env := plugins.PluginEnv{
		ClusterID: req.ClusterID,
		Nodes: []plugins.PluginNode{
			{
				HostID:    req.Node.HostID,
				Address:   req.Node.Address,
				AgentPort: req.Node.AgentPort,
				MySQLPort: req.Node.MySQLPort,
				Role:      "replica",
				DataDir:   req.Node.DataDir,
				Basedir:   req.Node.Basedir,
			},
		},
		Credentials: *creds,
	}

	if _, err := s.pluginExec.RunExecute(ctx, kernelPluginName, env, nil); err != nil {
		return &RebuildResult{
			ClusterID:  req.ClusterID,
			InstanceID: req.InstanceID,
			Status:     "failed",
			Message:    fmt.Sprintf("rebuild failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, err
	}

	// L3: Record audit log for rebuild operation
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Operation:    "rebuild",
			ResourceType: "instance",
			ResourceID:   req.InstanceID,
			Action:       fmt.Sprintf("rebuild node %s", req.Node.Address),
			Details:      fmt.Sprintf("arch=%s duration_ms=%d", req.ArchType, time.Since(start).Milliseconds()),
			Result:       "success",
		})
	}

	return &RebuildResult{
		ClusterID:  req.ClusterID,
		InstanceID: req.InstanceID,
		Status:     "completed",
		Message:    fmt.Sprintf("Node %s rebuilt as replica", req.Node.Address),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
