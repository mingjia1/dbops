package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type TopologyService struct {
	repo InstanceRepositoryInterface
}

func NewTopologyService(repo InstanceRepositoryInterface) *TopologyService {
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
		topology.SlaveIDs = parseTopologySlaveIDs(instance.Topology.SlaveIDs)

		if instance.Status.Role != "" {
			topology.Role = instance.Status.Role
		}
	} else if instance.Status.Role != "" {
		topology.Role = instance.Status.Role
	}

	return topology, nil
}

func parseTopologySlaveIDs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	var slaveIDs []string
	if err := json.Unmarshal([]byte(value), &slaveIDs); err == nil {
		return compactTopologyIDs(slaveIDs)
	}
	return compactTopologyIDs(strings.Split(value, ","))
}

func compactTopologyIDs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (s *TopologyService) GetClusterTopology(ctx context.Context, clusterID string) (*ClusterTopologyResponse, error) {
	instances, err := s.repo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("cluster not found or has no instances")
	}

	// 过滤掉已经被清除集群关联的实例（即已销毁集群的残留）
	var validInstances []InstanceTopologyResponse
	for _, inst := range instances {
		// 跳过已经没有集群ID的实例
		if inst.ClusterID == "" {
			log.Printf("INFO: skipping instance %s with empty cluster_id in topology query", inst.ID)
			continue
		}

		topologyInst, err := s.GetInstanceTopology(ctx, inst.ID)
		if err != nil {
			log.Printf("WARN: failed to get topology for instance %s: %v", inst.ID, err)
			continue
		}
		validInstances = append(validInstances, *topologyInst)
	}

	// 如果所有实例都已被清除，返回错误
	if len(validInstances) == 0 {
		return nil, fmt.Errorf("cluster has no valid instances (possibly destroyed)")
	}

	inferClusterTopology(validInstances)
	replicationMode := "async"
	for _, inst := range validInstances {
		if inst.ReplicationMode != "" {
			replicationMode = inst.ReplicationMode
			break
		}
	}

	return &ClusterTopologyResponse{
		ClusterID:       clusterID,
		ReplicationMode: replicationMode,
		Instances:       validInstances,
	}, nil
}

func (s *TopologyService) BuildTopologyGraph(ctx context.Context, clusterID string) (*TopologyGraph, error) {
	clusterTopology, err := s.GetClusterTopology(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster topology: %w", err)
	}

	var nodes []TopologyNode
	nodeIDs := make(map[string]bool)
	instanceMap := make(map[string]*models.Instance)

	// 预加载所有实例信息到map中
	for _, topologyInst := range clusterTopology.Instances {
		inst, err := s.repo.GetByID(ctx, topologyInst.InstanceID)
		if err != nil {
			log.Printf("WARN: topology node %s has no matching instance row, skipping: %v", topologyInst.InstanceID, err)
			continue
		}
		// 再次确认实例仍属于该集群（防止并发修改）
		if inst.ClusterID != clusterID {
			log.Printf("INFO: instance %s no longer belongs to cluster %s, skipping", inst.ID, clusterID)
			continue
		}
		instanceMap[inst.ID] = inst
	}

	// 构建节点列表（只包含有效实例）
	for _, topologyInst := range clusterTopology.Instances {
		inst, exists := instanceMap[topologyInst.InstanceID]
		if !exists {
			continue
		}

		node := TopologyNode{
			ID:        topologyInst.InstanceID,
			Name:      inst.Name,
			Role:      normalizeDisplayRole(topologyInst.Role),
			Status:    "unknown",
			ClusterID: topologyInst.ClusterID,
		}

		// 优先使用健康状态，其次使用运行状态
		if inst.Status.HealthStatus != "" {
			node.Status = inst.Status.HealthStatus
		} else if inst.Status.RunStatus != "" {
			node.Status = inst.Status.RunStatus
		}

		nodeIDs[node.ID] = true
		nodes = append(nodes, node)
	}

	// 如果没有有效节点，返回空图
	if len(nodes) == 0 {
		return &TopologyGraph{
			Nodes: []TopologyNode{},
			Edges: []TopologyEdge{},
		}, nil
	}

	// 构建边列表（只连接存在的节点）
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
		// 只处理仍在instanceMap中的实例
		if _, exists := instanceMap[topologyInst.InstanceID]; !exists {
			continue
		}

		if topologyInst.MasterID != "" {
			edge := TopologyEdge{
				SourceID: topologyInst.MasterID,
				TargetID: topologyInst.InstanceID,
				Type:     "replication",
				Label:    normalizeReplicationLabel(topologyInst.ReplicationMode),
			}
			addEdge(edge)
		}

		for _, slaveID := range topologyInst.SlaveIDs {
			edge := TopologyEdge{
				SourceID: topologyInst.InstanceID,
				TargetID: slaveID,
				Type:     "replication",
				Label:    normalizeReplicationLabel(topologyInst.ReplicationMode),
			}
			addEdge(edge)
		}
	}

	return &TopologyGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// normalizeDisplayRole 规范化角色显示名称
func normalizeDisplayRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "slave", "secondary":
		return "replica"
	case "primary_master":
		return "primary"
	case "":
		return "unknown"
	default:
		return role
	}
}

// normalizeReplicationLabel 规范化复制关系标签
func normalizeReplicationLabel(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "async"
	}
	return mode
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
