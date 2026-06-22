package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type ScaleService struct {
	orchestrator *DeployOrchestrator
	instRepo     InstanceRepositoryInterface
	pluginExec   *plugins.Executor
}

func NewScaleService(orchestrator *DeployOrchestrator, instRepo InstanceRepositoryInterface, pluginExec *plugins.Executor) *ScaleService {
	return &ScaleService{
		orchestrator: orchestrator,
		instRepo:     instRepo,
		pluginExec:   pluginExec,
	}
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

	for _, node := range req.NewNodes {
		env := plugins.PluginEnv{
			ClusterID: req.ClusterID,
			Nodes: []plugins.PluginNode{
				{
					HostID:    node.HostID,
					Address:   node.Address,
					AgentPort: node.AgentPort,
					MySQLPort: node.MySQLPort,
					Role:      "replica",
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

		archPluginName := req.ArchType + "-addon"
		joinEnv := plugins.PluginEnv{
			ClusterID: req.ClusterID,
			Nodes: []plugins.PluginNode{
				{
					HostID:    node.HostID,
					Address:   node.Address,
					AgentPort: node.AgentPort,
					MySQLPort: node.MySQLPort,
					Role:      "replica",
					DataDir:   node.DataDir,
					Basedir:   node.Basedir,
				},
			},
			Credentials: *creds,
		}

		if _, err := s.pluginExec.RunExecute(ctx, archPluginName, joinEnv, nil); err != nil {
			log.Printf("WARN: arch join failed for %s: %v", node.Address, err)
		}

		instID := uuid.New().String()
		if s.instRepo != nil {
			inst := &models.Instance{
				ID:        instID,
				Name:      fmt.Sprintf("replica-%s", node.Address),
				ClusterID: req.ClusterID,
			}
			if err := s.instRepo.Create(ctx, inst); err != nil {
				log.Printf("WARN: failed to persist instance: %v", err)
			}
		}
		instIDs = append(instIDs, instID)
	}

	return &ScaleOutResult{
		ClusterID:  req.ClusterID,
		NewInstIDs: instIDs,
		Status:     "completed",
		Message:    fmt.Sprintf("%d new nodes added to cluster", len(req.NewNodes)),
	}, nil
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
		archPluginName := req.ArchType + "-addon"
		leaveEnv := plugins.PluginEnv{
			ClusterID: req.ClusterID,
			Nodes:     []plugins.PluginNode{{Address: req.RemoveNodeID}},
		}
		if err := s.pluginExec.RunLeave(ctx, archPluginName, leaveEnv, plugins.PluginNode{}); err != nil {
			log.Printf("WARN: arch leave failed for %s: %v", req.RemoveNodeID, err)
		}
	}

	if s.instRepo != nil {
		if err := s.instRepo.Delete(ctx, req.RemoveNodeID); err != nil {
			return nil, fmt.Errorf("delete instance: %w", err)
		}
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

	return &RebuildResult{
		ClusterID:  req.ClusterID,
		InstanceID: req.InstanceID,
		Status:     "completed",
		Message:    fmt.Sprintf("Node %s rebuilt as replica", req.Node.Address),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
