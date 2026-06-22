package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type DeployOrchestrator struct {
	pluginRegistry   *plugins.Registry
	pluginExecutor   *plugins.Executor
	credentialVault  *CredentialVault
	instRepo         InstanceRepositoryInterface
}

func NewDeployOrchestrator(
	registry *plugins.Registry,
	vault *CredentialVault,
	instRepo InstanceRepositoryInterface,
) *DeployOrchestrator {
	return &DeployOrchestrator{
		pluginRegistry:  registry,
		pluginExecutor:  plugins.NewExecutor(registry),
		credentialVault: vault,
		instRepo:        instRepo,
	}
}

type DeployOrchestratorRequest struct {
	ClusterID   string
	ClusterName string
	Flavor      string
	ArchType    string
	EncKey      string
	Nodes       []OrchestratorNode
	Options     map[string]interface{}
}

type OrchestratorNode struct {
	HostID    string
	Address   string
	AgentPort int
	MySQLPort int
	Role      string
	DataDir   string
	Basedir   string
	ServerID  int
}

type OrchestratorResult struct {
	ClusterID   string        `json:"cluster_id"`
	Status      string        `json:"status"`
	Message     string        `json:"message"`
	Phases      []PhaseResult `json:"phases"`
	InstanceIDs []string      `json:"instance_ids"`
}

type PhaseResult struct {
	Phase      string `json:"phase"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DurationMs int64  `json:"duration_ms"`
}

func (o *DeployOrchestrator) Run(ctx context.Context, req DeployOrchestratorRequest) (*OrchestratorResult, error) {
	if len(req.Nodes) == 0 {
		return nil, fmt.Errorf("at least one node is required")
	}
	if req.ClusterID == "" {
		req.ClusterID = uuid.New().String()
	}

	result := &OrchestratorResult{
		ClusterID: req.ClusterID,
		Phases:    make([]PhaseResult, 0, 3),
	}

	creds, err := o.ensureCredentials(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ensure credentials: %w", err)
	}

	phase1, primaryInstID, err := o.phaseTemplateBuild(ctx, req, creds)
	result.Phases = append(result.Phases, *phase1)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Phase 1 failed: %v", err)
		return result, err
	}
	result.InstanceIDs = append(result.InstanceIDs, primaryInstID)

	phase2, replicaInstIDs, err := o.phaseReplicate(ctx, req, creds)
	result.Phases = append(result.Phases, *phase2)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Phase 2 failed: %v", err)
		return result, err
	}
	result.InstanceIDs = append(result.InstanceIDs, replicaInstIDs...)

	phase3, err := o.phaseAssemble(ctx, req, creds)
	result.Phases = append(result.Phases, *phase3)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Phase 3 failed: %v", err)
		return result, err
	}

	result.Status = "completed"
	result.Message = fmt.Sprintf("Cluster %s deployed with %d nodes", req.ClusterID, len(req.Nodes))
	return result, nil
}

func (o *DeployOrchestrator) ensureCredentials(ctx context.Context, req DeployOrchestratorRequest) (*plugins.CredentialSet, error) {
	if req.EncKey == "" {
		return nil, fmt.Errorf("encryption key is required")
	}

	vault := NewCredentialVault(o.credentialVault.repo, req.EncKey)

	creds := &plugins.CredentialSet{
		RootUser:    "root",
		ReplUser:    "repl",
		MonitorUser: "monitor",
	}

	rootPass, err := GenerateSecurePassword(24)
	if err != nil {
		return nil, err
	}
	creds.RootPassword = rootPass

	replPass, err := GenerateSecurePassword(24)
	if err != nil {
		return nil, err
	}
	creds.ReplPassword = replPass

	monitorPass, err := GenerateSecurePassword(24)
	if err != nil {
		return nil, err
	}
	creds.MonitorPassword = monitorPass

	if err := vault.SetCredential(ctx, req.ClusterID, "root", "root", rootPass); err != nil {
		return nil, err
	}
	if err := vault.SetCredential(ctx, req.ClusterID, "repl", "repl", replPass); err != nil {
		return nil, err
	}
	if err := vault.SetCredential(ctx, req.ClusterID, "monitor", "monitor", monitorPass); err != nil {
		return nil, err
	}

	return creds, nil
}

func (o *DeployOrchestrator) phaseTemplateBuild(ctx context.Context, req DeployOrchestratorRequest, creds *plugins.CredentialSet) (*PhaseResult, string, error) {
	start := time.Now()
	primary := req.Nodes[0]

	kernelPluginName := req.Flavor + "-core"
	env := plugins.PluginEnv{
		ClusterID: req.ClusterID,
		Nodes: []plugins.PluginNode{
			{
				HostID:    primary.HostID,
				Address:   primary.Address,
				AgentPort: primary.AgentPort,
				MySQLPort: primary.MySQLPort,
				Role:      "primary",
				DataDir:   primary.DataDir,
				Basedir:   primary.Basedir,
			},
		},
		Credentials: *creds,
	}

	if _, err := o.pluginExecutor.RunExecute(ctx, kernelPluginName, env, nil); err != nil {
		return &PhaseResult{Phase: "template_build", Status: "failed", Message: err.Error(), DurationMs: time.Since(start).Milliseconds()},
			"", err
	}

	instID := uuid.New().String()
	inst := &models.Instance{
		ID:        instID,
		Name:      fmt.Sprintf("%s-primary", req.ClusterName),
		ClusterID: req.ClusterID,
	}
	if o.instRepo != nil {
		if err := o.instRepo.Create(ctx, inst); err != nil {
			log.Printf("WARN: failed to persist primary instance: %v", err)
		}
	}

	return &PhaseResult{
		Phase:      "template_build",
		Status:     "completed",
		Message:    fmt.Sprintf("Primary node deployed on %s:%d", primary.Address, primary.MySQLPort),
		DurationMs: time.Since(start).Milliseconds(),
	}, instID, nil
}

func (o *DeployOrchestrator) phaseReplicate(ctx context.Context, req DeployOrchestratorRequest, creds *plugins.CredentialSet) (*PhaseResult, []string, error) {
	start := time.Now()

	if len(req.Nodes) <= 1 {
		return &PhaseResult{Phase: "replicate", Status: "skipped", Message: "single node, no replicas needed"}, nil, nil
	}

	replicaNodes := req.Nodes[1:]
	var mu sync.Mutex
	var wg sync.WaitGroup
	var instIDs []string
	var firstErr error

	for _, node := range replicaNodes {
		wg.Add(1)
		go func(n OrchestratorNode) {
			defer wg.Done()

			kernelPluginName := req.Flavor + "-core"
			env := plugins.PluginEnv{
				ClusterID: req.ClusterID,
				Nodes: []plugins.PluginNode{
					{
						HostID:    n.HostID,
						Address:   n.Address,
						AgentPort: n.AgentPort,
						MySQLPort: n.MySQLPort,
						Role:      "replica",
						DataDir:   n.DataDir,
						Basedir:   n.Basedir,
					},
				},
				Credentials: *creds,
			}

			if _, err := o.pluginExecutor.RunExecute(ctx, kernelPluginName, env, nil); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("node %s: %w", n.Address, err)
				}
				mu.Unlock()
				return
			}

			instID := uuid.New().String()
			if o.instRepo != nil {
				inst := &models.Instance{
					ID:        instID,
					Name:      fmt.Sprintf("%s-replica-%d", req.ClusterName, n.ServerID),
					ClusterID: req.ClusterID,
				}
				if err := o.instRepo.Create(ctx, inst); err != nil {
					log.Printf("WARN: failed to persist replica instance: %v", err)
				}
			}

			mu.Lock()
			instIDs = append(instIDs, instID)
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	if firstErr != nil {
		return &PhaseResult{Phase: "replicate", Status: "failed", Message: firstErr.Error()}, nil, firstErr
	}

	return &PhaseResult{
		Phase:      "replicate",
		Status:     "completed",
		Message:    fmt.Sprintf("%d replica nodes deployed concurrently", len(replicaNodes)),
		DurationMs: time.Since(start).Milliseconds(),
	}, instIDs, nil
}

func (o *DeployOrchestrator) phaseAssemble(ctx context.Context, req DeployOrchestratorRequest, creds *plugins.CredentialSet) (*PhaseResult, error) {
	start := time.Now()

	if req.ArchType == "" || req.ArchType == "single" {
		return &PhaseResult{
			Phase:      "assemble",
			Status:     "skipped",
			Message:    "single instance, no assembly needed",
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	archPluginName := req.ArchType + "-addon"
	env := plugins.PluginEnv{
		ClusterID: req.ClusterID,
		Nodes:     o.buildPluginNodes(req.Nodes),
		Credentials: *creds,
	}

	result, err := o.pluginExecutor.RunExecute(ctx, archPluginName, env, req.Options)
	if err != nil {
		return &PhaseResult{
			Phase:      "assemble",
			Status:     "failed",
			Message:    err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}, err
	}

	msg := "cluster assembled"
	if result != nil {
		msg = result.Message
	}

	return &PhaseResult{
		Phase:      "assemble",
		Status:     "completed",
		Message:    msg,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func (o *DeployOrchestrator) buildPluginNodes(nodes []OrchestratorNode) []plugins.PluginNode {
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
