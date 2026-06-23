package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type ClusterLifecycleService struct {
	orchestrator   *DeployOrchestrator
	pluginExec     *plugins.Executor
	credentialVault *CredentialVault
	instRepo       InstanceRepositoryInterface
}

func NewClusterLifecycleService(
	orchestrator *DeployOrchestrator,
	pluginExec *plugins.Executor,
	vault *CredentialVault,
	instRepo InstanceRepositoryInterface,
) *ClusterLifecycleService {
	return &ClusterLifecycleService{
		orchestrator:    orchestrator,
		pluginExec:      pluginExec,
		credentialVault: vault,
		instRepo:        instRepo,
	}
}

type DestroyRequest struct {
	ClusterID string
	Flavor    string
	ArchType  string
	EncKey    string
	Nodes     []OrchestratorNode
}

type DestroyResult struct {
	ClusterID string `json:"cluster_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	DurationMs int64 `json:"duration_ms"`
}

func (s *ClusterLifecycleService) DestroyCluster(ctx context.Context, req DestroyRequest) (*DestroyResult, error) {
	start := time.Now()

	result := &DestroyResult{ClusterID: req.ClusterID}

	env := plugins.PluginEnv{
		ClusterID: req.ClusterID,
		Nodes:     buildPluginNodesFromOrchestrator(req.Nodes),
	}

	if req.ArchType != "" && req.ArchType != "single" {
		archPluginName := req.ArchType + "-addon"
		if err := s.pluginExec.RunTeardown(ctx, archPluginName, env); err != nil {
			log.Printf("WARN: arch teardown failed: %v", err)
		}
	}

	kernelPluginName := req.Flavor + "-core"
	if err := s.pluginExec.RunTeardown(ctx, kernelPluginName, env); err != nil {
		log.Printf("WARN: kernel teardown failed: %v", err)
	}

	if err := s.credentialVault.DeleteClusterCredentials(ctx, req.ClusterID); err != nil {
		log.Printf("WARN: credential cleanup failed: %v", err)
	}

	if s.instRepo != nil {
		instances, err := s.instRepo.ListByClusterID(ctx, req.ClusterID)
		if err == nil {
			for _, inst := range instances {
				if err := s.instRepo.Delete(ctx, inst.ID); err != nil {
					log.Printf("WARN: failed to delete instance %s: %v", inst.ID, err)
				}
			}
		}
	}

	result.Status = "completed"
	result.Message = fmt.Sprintf("Cluster %s destroyed", req.ClusterID)
	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

type RebuildClusterRequest struct {
	OriginalReq DeployOrchestratorRequest
}

type RebuildClusterResult struct {
	ClusterID string             `json:"cluster_id"`
	Status    string             `json:"status"`
	Message   string             `json:"message"`
	Deploy    *OrchestratorResult `json:"deploy,omitempty"`
}

func (s *ClusterLifecycleService) RebuildCluster(ctx context.Context, req RebuildClusterRequest) (*RebuildClusterResult, error) {
	destroyReq := DestroyRequest{
		ClusterID: req.OriginalReq.ClusterID,
		Flavor:    req.OriginalReq.Flavor,
		ArchType:  req.OriginalReq.ArchType,
		EncKey:    req.OriginalReq.EncKey,
		Nodes:     req.OriginalReq.Nodes,
	}

	_, err := s.DestroyCluster(ctx, destroyReq)
	if err != nil {
		return &RebuildClusterResult{
			ClusterID: req.OriginalReq.ClusterID,
			Status:    "failed",
			Message:   fmt.Sprintf("destroy phase failed: %v", err),
		}, err
	}

	deployResult, err := s.orchestrator.Run(ctx, req.OriginalReq)
	if err != nil {
		return &RebuildClusterResult{
			ClusterID: req.OriginalReq.ClusterID,
			Status:    "failed",
			Message:   fmt.Sprintf("deploy phase failed: %v", err),
		}, err
	}

	return &RebuildClusterResult{
		ClusterID: req.OriginalReq.ClusterID,
		Status:    deployResult.Status,
		Message:   fmt.Sprintf("Cluster %s rebuilt successfully", req.OriginalReq.ClusterID),
		Deploy:    deployResult,
	}, nil
}

func buildPluginNodesFromOrchestrator(nodes []OrchestratorNode) []plugins.PluginNode {
	out := make([]plugins.PluginNode, len(nodes))
	for i, n := range nodes {
		out[i] = plugins.PluginNode{
			HostID:    n.HostID,
			Address:   n.Address,
			AgentPort: n.AgentPort,
			MySQLPort: n.MySQLPort,
			Role:      n.Role,
			DataDir:   n.DataDir,
			Basedir:   n.Basedir,
		}
	}
	return out
}
