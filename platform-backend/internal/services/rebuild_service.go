package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type RebuildService struct {
	pluginExec      *plugins.Executor
	credentialVault *CredentialVault
	instRepo        InstanceRepositoryInterface
}

func NewRebuildService(pluginExec *plugins.Executor, vault *CredentialVault, instRepo InstanceRepositoryInterface) *RebuildService {
	return &RebuildService{
		pluginExec:      pluginExec,
		credentialVault: vault,
		instRepo:        instRepo,
	}
}

type RebuildServiceRequest struct {
	ClusterID   string
	InstanceID  string
	Flavor      string
	ArchType    string
	EncKey      string
	Node        OrchestratorNode
}

type RebuildServiceResult struct {
	ClusterID  string `json:"cluster_id"`
	InstanceID string `json:"instance_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DurationMs int64  `json:"duration_ms"`
}

func (s *RebuildService) RebuildNode(ctx context.Context, req RebuildServiceRequest) (*RebuildServiceResult, error) {
	start := time.Now()

	creds, err := s.loadCredentials(ctx, req.ClusterID, req.EncKey)
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
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

	teardownEnv := plugins.PluginEnv{
		ClusterID: req.ClusterID,
		Nodes:     env.Nodes,
	}
	if err := s.pluginExec.RunTeardown(ctx, kernelPluginName, teardownEnv); err != nil {
		log.Printf("WARN: teardown failed for %s: %v", req.Node.Address, err)
	}

	if _, err := s.pluginExec.RunExecute(ctx, kernelPluginName, env, nil); err != nil {
		return &RebuildServiceResult{
			ClusterID:  req.ClusterID,
			InstanceID: req.InstanceID,
			Status:     "failed",
			Message:    fmt.Sprintf("rebuild failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, err
	}

	if req.ArchType != "" && req.ArchType != "single" {
		archPluginName := req.ArchType + "-addon"
		if err := s.pluginExec.RunJoin(ctx, archPluginName, env, env.Nodes[0]); err != nil {
			log.Printf("WARN: arch join failed for %s: %v", req.Node.Address, err)
		}
	}

	return &RebuildServiceResult{
		ClusterID:  req.ClusterID,
		InstanceID: req.InstanceID,
		Status:     "completed",
		Message:    fmt.Sprintf("Node %s rebuilt successfully", req.Node.Address),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func (s *RebuildService) loadCredentials(ctx context.Context, clusterID, encKey string) (*plugins.CredentialSet, error) {
	if encKey == "" {
		return nil, fmt.Errorf("encryption key is required")
	}
	vault := NewCredentialVault(s.credentialVault.repo, encKey)

	creds := &plugins.CredentialSet{
		RootUser:    "root",
		ReplUser:    "repl",
		MonitorUser: "monitor",
	}

	if rootPass, err := vault.GetDecryptedPassword(ctx, clusterID, "root"); err == nil {
		creds.RootPassword = rootPass
	}
	if replPass, err := vault.GetDecryptedPassword(ctx, clusterID, "repl"); err == nil {
		creds.ReplPassword = replPass
	}
	if monitorPass, err := vault.GetDecryptedPassword(ctx, clusterID, "monitor"); err == nil {
		creds.MonitorPassword = monitorPass
	}

	return creds, nil
}
