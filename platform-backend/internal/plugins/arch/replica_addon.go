package arch

import (
	"context"
	"fmt"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type ReplicaAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
	plugins.DefaultPluginMethods
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

func (p *ReplicaAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
