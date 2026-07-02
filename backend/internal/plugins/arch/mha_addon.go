package arch

import (
	"context"
	"fmt"
	"log"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type MHAAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func (p *MHAAddonPlugin) Join(ctx context.Context, env plugins.PluginEnv, newNode plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"task_id":   fmt.Sprintf("mha-join-%s-%s", env.ClusterID, newNode.Address),
		"repl_user": env.Credentials.ReplUser,
		"repl_pass": env.Credentials.ReplPassword,
	}
	_, err := p.agentCaller(ctx, newNode.Address, newNode.AgentPort, "/api/v1/replication/setup", payload)
	return err
}

func (p *MHAAddonPlugin) Leave(ctx context.Context, env plugins.PluginEnv, node plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"task_id": fmt.Sprintf("mha-leave-%s-%s", env.ClusterID, node.Address),
		"action":  "leave",
	}
	_, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/replication/teardown", payload)
	if err != nil {
		log.Printf("WARN: MHA leave failed for %s: %v — MHA manager config needs manual update", node.Address, err)
	}
	return err
}

func NewMHAAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *MHAAddonPlugin {
	return &MHAAddonPlugin{agentCaller: agentCaller}
}

func (p *MHAAddonPlugin) Name() string    { return "mha-addon" }
func (p *MHAAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeArch }
func (p *MHAAddonPlugin) Version() string { return "1.0.0" }

func (p *MHAAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 2 {
		return fmt.Errorf("MHA addon requires at least 2 nodes")
	}
	return nil
}

func (p *MHAAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}

	var managerNode *plugins.PluginNode
	var primaryNode *plugins.PluginNode
	var replicaNodes []plugins.PluginNode

	for _, n := range env.Nodes {
		switch n.Role {
		case "manager":
			node := n
			managerNode = &node
		case "primary", "master":
			node := n
			primaryNode = &node
		default:
			replicaNodes = append(replicaNodes, n)
		}
	}

	if primaryNode == nil {
		return nil, fmt.Errorf("no primary node found for MHA")
	}

	for _, replica := range replicaNodes {
		payload := map[string]interface{}{
			"task_id":       fmt.Sprintf("mha-repl-%s", env.ClusterID),
			"master_host":   primaryNode.Address,
			"master_port":   primaryNode.MySQLPort,
			"repl_user":     env.Credentials.ReplUser,
			"repl_pass":     env.Credentials.ReplPassword,
		}
		if _, err := p.agentCaller(ctx, replica.Address, replica.AgentPort, "/api/v1/replication/setup", payload); err != nil {
			return nil, fmt.Errorf("setup replication on %s: %w", replica.Address, err)
		}
	}

	if managerNode != nil {
		mhaConfig := map[string]interface{}{
			"task_id":     fmt.Sprintf("mha-manager-%s", env.ClusterID),
			"master_host": primaryNode.Address,
			"master_port": primaryNode.MySQLPort,
			"replicas":    buildReplicaList(replicaNodes),
			"ssh_user":    env.Credentials.RootUser,
		}
		if _, err := p.agentCaller(ctx, managerNode.Address, managerNode.AgentPort, "/api/v1/mha/setup", mhaConfig); err != nil {
			return nil, fmt.Errorf("setup MHA manager on %s: %w", managerNode.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("MHA cluster assembled: 1 manager + 1 primary + %d replicas", len(replicaNodes)),
	}, nil
}

func buildReplicaList(nodes []plugins.PluginNode) []map[string]interface{} {
	out := make([]map[string]interface{}, len(nodes))
	for i, n := range nodes {
		out[i] = map[string]interface{}{
			"host": n.Address,
			"port": n.MySQLPort,
		}
	}
	return out
}

func (p *MHAAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *MHAAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
