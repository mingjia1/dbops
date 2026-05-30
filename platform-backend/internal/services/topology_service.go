package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type TopologyService struct {
	repo *repositories.InstanceRepository
}

func NewTopologyService(repo *repositories.InstanceRepository) *TopologyService {
	return &TopologyService{repo: repo}
}

type InstanceTopologyResponse struct {
	InstanceID      string   `json:"instance_id"`
	ClusterID       string   `json:"cluster_id"`
	MasterID        string   `json:"master_id"`
	SlaveIDs        []string `json:"slave_ids"`
	ReplicationMode string   `json:"replication_mode"`
	Role            string   `json:"role"`
}

type ClusterTopologyResponse struct {
	ClusterID       string                    `json:"cluster_id"`
	ReplicationMode string                    `json:"replication_mode"`
	Instances       []InstanceTopologyResponse `json:"instances"`
}

type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	ClusterID string `json:"cluster_id"`
}

type TopologyEdge struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Type     string `json:"type"`
	Label    string `json:"label"`
}

func (s *TopologyService) GetInstanceTopology(ctx context.Context, instanceID string) (*InstanceTopologyResponse, error) {
	instance, err := s.repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	topology := &InstanceTopologyResponse{
		InstanceID:      instance.ID,
		ClusterID:       instance.ClusterID,
		MasterID:        "",
		SlaveIDs:        []string{},
		ReplicationMode: "async",
		Role:            "master",
	}

	if instance.Topology.InstanceID != "" {
		topology.MasterID = instance.Topology.MasterID
		topology.ReplicationMode = instance.Topology.ReplicationMode
		
		if instance.Topology.SlaveIDs != "" {
			var slaveIDs []string
			if err := json.Unmarshal([]byte(instance.Topology.SlaveIDs), &slaveIDs); err == nil {
				topology.SlaveIDs = slaveIDs
			}
		}

		if instance.Status.Role != "" {
			topology.Role = instance.Status.Role
		}
	}

	return topology, nil
}

func (s *TopologyService) GetClusterTopology(ctx context.Context, clusterID string) (*ClusterTopologyResponse, error) {
	instances, err := s.repo.List(ctx, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	var clusterInstances []models.Instance
	for _, inst := range instances {
		if inst.ClusterID == clusterID {
			clusterInstances = append(clusterInstances, inst)
		}
	}

	if len(clusterInstances) == 0 {
		return nil, fmt.Errorf("cluster not found or has no instances")
	}

	replicationMode := "async"
	if clusterInstances[0].Topology.ReplicationMode != "" {
		replicationMode = clusterInstances[0].Topology.ReplicationMode
	}

	var topologyInstances []InstanceTopologyResponse
	for _, inst := range clusterInstances {
		topologyInst, err := s.GetInstanceTopology(ctx, inst.ID)
		if err != nil {
			continue
		}
		topologyInstances = append(topologyInstances, *topologyInst)
	}

	return &ClusterTopologyResponse{
		ClusterID:       clusterID,
		ReplicationMode: replicationMode,
		Instances:       topologyInstances,
	}, nil
}

func (s *TopologyService) BuildTopologyGraph(ctx context.Context, clusterID string) (*TopologyGraph, error) {
	clusterTopology, err := s.GetClusterTopology(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster topology: %w", err)
	}

	instances, err := s.repo.List(ctx, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	instanceMap := make(map[string]models.Instance)
	for _, inst := range instances {
		instanceMap[inst.ID] = inst
	}

	var nodes []TopologyNode
	for _, topologyInst := range clusterTopology.Instances {
		inst := instanceMap[topologyInst.InstanceID]
		node := TopologyNode{
			ID:        topologyInst.InstanceID,
			Name:      inst.Name,
			Role:      topologyInst.Role,
			Status:    "healthy",
			ClusterID: topologyInst.ClusterID,
		}
		if inst.Status.HealthStatus != "" {
			node.Status = inst.Status.HealthStatus
		}
		nodes = append(nodes, node)
	}

	var edges []TopologyEdge
	for _, topologyInst := range clusterTopology.Instances {
		if topologyInst.MasterID != "" {
			edge := TopologyEdge{
				SourceID: topologyInst.MasterID,
				TargetID: topologyInst.InstanceID,
				Type:     "replication",
				Label:    topologyInst.ReplicationMode,
			}
			edges = append(edges, edge)
		}

		for _, slaveID := range topologyInst.SlaveIDs {
			edge := TopologyEdge{
				SourceID: topologyInst.InstanceID,
				TargetID: slaveID,
				Type:     "replication",
				Label:    topologyInst.ReplicationMode,
			}
			edges = append(edges, edge)
		}
	}

	return &TopologyGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}