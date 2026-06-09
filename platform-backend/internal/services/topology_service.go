package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	ClusterID       string                     `json:"cluster_id"`
	ReplicationMode string                     `json:"replication_mode"`
	Instances       []InstanceTopologyResponse `json:"instances"`
}

type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
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
		Role:            "unknown",
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
	} else if instance.Status.Role != "" {
		topology.Role = instance.Status.Role
	}

	return topology, nil
}

func (s *TopologyService) GetClusterTopology(ctx context.Context, clusterID string) (*ClusterTopologyResponse, error) {
	instances, err := s.repo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("cluster not found or has no instances")
	}

	var topologyInstances []InstanceTopologyResponse
	for _, inst := range instances {
		topologyInst, err := s.GetInstanceTopology(ctx, inst.ID)
		if err != nil {
			continue
		}
		topologyInstances = append(topologyInstances, *topologyInst)
	}
	inferClusterTopology(topologyInstances)
	replicationMode := "async"
	for _, inst := range topologyInstances {
		if inst.ReplicationMode != "" {
			replicationMode = inst.ReplicationMode
			break
		}
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

	var nodes []TopologyNode
	nodeIDs := make(map[string]bool)
	for _, topologyInst := range clusterTopology.Instances {
		inst, err := s.repo.GetByID(ctx, topologyInst.InstanceID)
		if err != nil {
			// B5: instanceMap 拿不到时不要塞零值, 跳过该节点 + 记 warn,
			// 避免前端出现"未命名节点"假象.
			log.Printf("WARN: topology node %s has no matching instance row, skipping: %v", topologyInst.InstanceID, err)
			continue
		}
		node := TopologyNode{
			ID:        topologyInst.InstanceID,
			Name:      inst.Name,
			Role:      topologyInst.Role,
			Status:    "unknown",
			ClusterID: topologyInst.ClusterID,
		}
		if inst.Status.HealthStatus != "" {
			node.Status = inst.Status.HealthStatus
		} else if inst.Status.RunStatus != "" {
			node.Status = inst.Status.RunStatus
		}
		nodeIDs[node.ID] = true
		nodes = append(nodes, node)
	}

	var edges []TopologyEdge
	edgeKeys := make(map[string]bool)
	addEdge := func(edge TopologyEdge) {
		if !nodeIDs[edge.SourceID] || !nodeIDs[edge.TargetID] {
			return
		}
		key := edge.SourceID + "->" + edge.TargetID + ":" + edge.Type
		if edgeKeys[key] {
			return
		}
		edgeKeys[key] = true
		edges = append(edges, edge)
	}
	for _, topologyInst := range clusterTopology.Instances {
		if topologyInst.MasterID != "" {
			edge := TopologyEdge{
				SourceID: topologyInst.MasterID,
				TargetID: topologyInst.InstanceID,
				Type:     "replication",
				Label:    topologyInst.ReplicationMode,
			}
			addEdge(edge)
		}

		for _, slaveID := range topologyInst.SlaveIDs {
			edge := TopologyEdge{
				SourceID: topologyInst.InstanceID,
				TargetID: slaveID,
				Type:     "replication",
				Label:    topologyInst.ReplicationMode,
			}
			addEdge(edge)
		}
	}

	return &TopologyGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

func inferClusterTopology(instances []InstanceTopologyResponse) {
	if len(instances) <= 1 {
		return
	}
	hasExplicitRelation := false
	for _, inst := range instances {
		if inst.MasterID != "" || len(inst.SlaveIDs) > 0 {
			hasExplicitRelation = true
			break
		}
	}
	if hasExplicitRelation {
		return
	}

	primaryIndex := 0
	for i, inst := range instances {
		if topologyIsPrimaryRole(inst.Role) {
			primaryIndex = i
			break
		}
	}
	primaryID := instances[primaryIndex].InstanceID
	slaveIDs := make([]string, 0, len(instances)-1)
	for i := range instances {
		if instances[i].ReplicationMode == "" {
			instances[i].ReplicationMode = "async"
		}
		if i == primaryIndex {
			instances[i].Role = normalizeTopologyRole(instances[i].Role, "master")
			continue
		}
		instances[i].Role = normalizeTopologyRole(instances[i].Role, "replica")
		instances[i].MasterID = primaryID
		slaveIDs = append(slaveIDs, instances[i].InstanceID)
	}
	instances[primaryIndex].SlaveIDs = slaveIDs
}

func topologyIsPrimaryRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "master", "primary", "primary_master", "leader", "writer":
		return true
	default:
		return false
	}
}

func normalizeTopologyRole(role, fallback string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" || role == "unknown" {
		return fallback
	}
	if role == "slave" || role == "secondary" {
		return "replica"
	}
	return role
}
