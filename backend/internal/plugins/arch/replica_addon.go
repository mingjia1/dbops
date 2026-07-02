package arch

import (
	"context"
	"fmt"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type ReplicaAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func (p *ReplicaAddonPlugin) Join(ctx context.Context, env plugins.PluginEnv, newNode plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	var primary *plugins.PluginNode
	for _, n := range env.Nodes {
		if n.Role == "primary" || n.Role == "master" {
			node := n
			primary = &node
			break
		}
	}
	if primary == nil {
		return fmt.Errorf("no primary node found for join")
	}
	payload := map[string]interface{}{
		"task_id":      fmt.Sprintf("join-%s-%s", env.ClusterID, newNode.Address),
		"master_host":  primary.Address,
		"master_port":  primary.MySQLPort,
		"repl_user":    env.Credentials.ReplUser,
		"repl_pass":    env.Credentials.ReplPassword,
		"replica_host": newNode.Address,
		"replica_port": newNode.MySQLPort,
	}
	_, err := p.agentCaller(ctx, newNode.Address, newNode.AgentPort, "/api/v1/replication/setup", payload)
	return err
}

func (p *ReplicaAddonPlugin) Leave(ctx context.Context, env plugins.PluginEnv, node plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	// Find the primary node to provide as master info
	var primary *plugins.PluginNode
	for _, n := range env.Nodes {
		if n.Role == "primary" || n.Role == "master" {
			primaryNode := n
			primary = &primaryNode
			break
		}
	}
	if primary == nil {
		return fmt.Errorf("no primary node found for leave")
	}
	payload := map[string]interface{}{
		"task_id":      fmt.Sprintf("leave-%s-%s", env.ClusterID, node.Address),
		"master_host":  primary.Address,
		"master_port":  primary.MySQLPort,
		"replica_host": node.Address,
		"replica_port": node.MySQLPort,
	}
	_, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/replication/teardown", payload)
	return err
}

func NewReplicaAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *ReplicaAddonPlugin {
	return &ReplicaAddonPlugin{agentCaller: agentCaller}
}

func (p *ReplicaAddonPlugin) Name() string    { return "replica-addon" }
func (p *ReplicaAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeArch }
func (p *ReplicaAddonPlugin) Version() string { return "1.0.0" }

func (p *ReplicaAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 2 {
		return fmt.Errorf("replica addon requires at least 2 nodes (1 primary + 1 replica)")
	}
	if env.Credentials.ReplUser == "" || env.Credentials.ReplPassword == "" {
		return fmt.Errorf("replication credentials are required")
	}
	return nil
}

func (p *ReplicaAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}

	var primary *plugins.PluginNode
	var replicas []plugins.PluginNode
	for _, n := range env.Nodes {
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
	if len(replicas) == 0 {
		return nil, fmt.Errorf("no replica nodes found")
	}

	for _, replica := range replicas {
		payload := map[string]interface{}{
			"task_id":       fmt.Sprintf("assemble-%s-replica", env.ClusterID),
			"master_host":   primary.Address,
			"master_port":   primary.MySQLPort,
			"repl_user":     env.Credentials.ReplUser,
			"repl_pass":     env.Credentials.ReplPassword,
			"replica_host":  replica.Address,
			"replica_port":  replica.MySQLPort,
		}

		_, err := p.agentCaller(ctx, replica.Address, replica.AgentPort, "/api/v1/replication/setup", payload)
		if err != nil {
			return nil, fmt.Errorf("setup replication on %s: %w", replica.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("Replication established: 1 primary + %d replicas", len(replicas)),
	}, nil
}

func (p *ReplicaAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *ReplicaAddonPlugin) Teardown(ctx context.Context, env plugins.PluginEnv) error {
	if p.agentCaller == nil {
		return nil
	}
	var primary *plugins.PluginNode
	for _, n := range env.Nodes {
		if n.Role == "primary" || n.Role == "master" {
			primaryNode := n
			primary = &primaryNode
			break
		}
	}
	if primary == nil {
		return nil
	}
	var lastErr error
	for _, node := range env.Nodes {
		if node.Role == "primary" || node.Role == "master" {
			continue
		}
		payload := map[string]interface{}{
			"task_id":      fmt.Sprintf("teardown-%s-%s", env.ClusterID, node.Address),
			"master_host":  primary.Address,
			"master_port":  primary.MySQLPort,
			"replica_host": node.Address,
			"replica_port": node.MySQLPort,
		}
		if _, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/replication/teardown", payload); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
