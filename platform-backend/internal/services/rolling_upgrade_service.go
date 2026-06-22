package services

import (
	"context"
	"fmt"
	"log"
	"time"
)

type RollingUpgradeService struct {
	orchestrator  *DeployOrchestrator
	switchSvc     interface{ ExecuteRoleSwitch(ctx context.Context, req interface{}) (interface{}, error) }
	instRepo      InstanceRepositoryInterface
	pluginExec    interface{}
}

func NewRollingUpgradeService(orchestrator *DeployOrchestrator, instRepo InstanceRepositoryInterface) *RollingUpgradeService {
	return &RollingUpgradeService{
		orchestrator: orchestrator,
		instRepo:     instRepo,
	}
}

type RollingUpgradeRequest struct {
	ClusterID     string
	Flavor        string
	ArchType      string
	EncKey        string
	SourceVersion string
	TargetVersion string
	Nodes         []OrchestratorNode
}

type RollingUpgradeResult struct {
	ClusterID      string         `json:"cluster_id"`
	Status         string         `json:"status"`
	Message        string         `json:"message"`
	UpgradedNodes  int            `json:"upgraded_nodes"`
	TotalNodes     int            `json:"total_nodes"`
	Phases         []PhaseResult  `json:"phases"`
}

func (s *RollingUpgradeService) ExecuteRollingUpgrade(ctx context.Context, req RollingUpgradeRequest) (*RollingUpgradeResult, error) {
	if len(req.Nodes) < 2 {
		return nil, fmt.Errorf("rolling upgrade requires at least 2 nodes")
	}

	result := &RollingUpgradeResult{
		ClusterID: req.ClusterID,
		TotalNodes: len(req.Nodes),
		Phases:    make([]PhaseResult, 0),
	}

	var primary *OrchestratorNode
	var replicas []OrchestratorNode
	for _, n := range req.Nodes {
		if n.Role == "primary" || n.Role == "master" {
			node := n
			primary = &node
		} else {
			replicas = append(replicas, n)
		}
	}
	if primary == nil {
		return nil, fmt.Errorf("no primary node found")
	}

	for i, replica := range replicas {
		start := time.Now()
		log.Printf("INFO: upgrading replica %d/%d: %s", i+1, len(replicas), replica.Address)

		singleReq := DeployOrchestratorRequest{
			ClusterID:   req.ClusterID,
			ClusterName: fmt.Sprintf("upgrade-replica-%d", i),
			Flavor:      req.Flavor,
			ArchType:    "single",
			EncKey:      req.EncKey,
			Nodes:       []OrchestratorNode{replica},
		}
		if _, err := s.orchestrator.Run(ctx, singleReq); err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("upgrade replica %s failed: %v", replica.Address, err)
			return result, err
		}

		result.UpgradedNodes++
		result.Phases = append(result.Phases, PhaseResult{
			Phase:      fmt.Sprintf("upgrade-replica-%d", i),
			Status:     "completed",
			Message:    fmt.Sprintf("Replica %s upgraded", replica.Address),
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	log.Printf("INFO: promoting new primary and upgrading old primary %s", primary.Address)

	start := time.Now()
	singleReq := DeployOrchestratorRequest{
		ClusterID:   req.ClusterID,
		ClusterName: "upgrade-old-primary",
		Flavor:      req.Flavor,
		ArchType:    "single",
		EncKey:      req.EncKey,
		Nodes:       []OrchestratorNode{*primary},
	}
	if _, err := s.orchestrator.Run(ctx, singleReq); err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("upgrade primary %s failed: %v", primary.Address, err)
		return result, err
	}
	result.UpgradedNodes++
	result.Phases = append(result.Phases, PhaseResult{
		Phase:      "upgrade-primary",
		Status:     "completed",
		Message:    fmt.Sprintf("Primary %s upgraded", primary.Address),
		DurationMs: time.Since(start).Milliseconds(),
	})

	result.Status = "completed"
	result.Message = fmt.Sprintf("Rolling upgrade completed: %d/%d nodes upgraded", result.UpgradedNodes, result.TotalNodes)
	return result, nil
}
