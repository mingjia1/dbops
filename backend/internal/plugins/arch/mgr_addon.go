package arch

import (
	"context"
	"fmt"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type MGRAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func (p *MGRAddonPlugin) Join(ctx context.Context, env plugins.PluginEnv, newNode plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	// H2: During ScaleOut, derive per-node local_port from node count
	// to avoid port conflicts. Existing nodes' ports are in env.Custom.
	localPort := 33061 + len(env.Nodes)
	if v, ok := env.Custom["local_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			localPort = port + len(env.Nodes)
		}
	}
	groupName := env.ClusterID
	if v, ok := env.Custom["group_name"]; ok {
		if name, ok := v.(string); ok && name != "" {
			groupName = name
		}
	}
	seeds := []string{fmt.Sprintf("%s:%d", newNode.Address, localPort)}
	for _, n := range env.Nodes {
		seeds = append(seeds, fmt.Sprintf("%s:%d", n.Address, localPort))
	}
	payload := map[string]interface{}{
		"task_id":       fmt.Sprintf("mgr-join-%s-%s", env.ClusterID, newNode.Address),
		"action":        "join",
		"group_name":    groupName,
		"local_address": fmt.Sprintf("%s:%d", newNode.Address, localPort),
		"group_seeds":   seeds,
		"bootstrap":     false,
	}
	_, err := p.agentCaller(ctx, newNode.Address, newNode.AgentPort, "/api/v1/mgr/setup", payload)
	return err
}

func (p *MGRAddonPlugin) Leave(ctx context.Context, env plugins.PluginEnv, node plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"task_id": fmt.Sprintf("mgr-leave-%s-%s", env.ClusterID, node.Address),
		"action":  "leave",
	}
	_, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/mgr/setup", payload)
	return err
}

func NewMGRAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *MGRAddonPlugin {
	return &MGRAddonPlugin{agentCaller: agentCaller}
}

func (p *MGRAddonPlugin) Name() string    { return "mgr-addon" }
func (p *MGRAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeArch }
func (p *MGRAddonPlugin) Version() string { return "1.0.0" }

func (p *MGRAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 1 {
		return fmt.Errorf("MGR addon requires at least 1 node")
	}
	return nil
}

func (p *MGRAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}
	localPort := 33061
	if v, ok := env.Custom["local_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			localPort = port
		}
	}
	groupName := env.ClusterID
	if v, ok := env.Custom["group_name"]; ok {
		if name, ok := v.(string); ok && name != "" {
			groupName = name
		}
	}

	var primary *plugins.PluginNode
	var members []plugins.PluginNode
	for _, n := range env.Nodes {
		if n.Role == "primary" || n.Role == "master" {
			node := n
			primary = &node
		} else {
			members = append(members, n)
		}
	}
	if primary == nil {
		primary = &env.Nodes[0]
		members = env.Nodes[1:]
	}

	seeds := make([]string, 0, len(env.Nodes))
	for _, n := range env.Nodes {
		seeds = append(seeds, fmt.Sprintf("%s:%d", n.Address, localPort))
	}

	bootstrapPayload := map[string]interface{}{
		"task_id":       fmt.Sprintf("mgr-bootstrap-%s", env.ClusterID),
		"action":        "bootstrap",
		"group_name":    groupName,
		"local_address": fmt.Sprintf("%s:%d", primary.Address, localPort),
		"group_seeds":   seeds,
		"bootstrap":     true,
	}
	if _, err := p.agentCaller(ctx, primary.Address, primary.AgentPort, "/api/v1/mgr/setup", bootstrapPayload); err != nil {
		return nil, fmt.Errorf("MGR bootstrap on %s: %w", primary.Address, err)
	}

	for _, member := range members {
		joinPayload := map[string]interface{}{
			"task_id":       fmt.Sprintf("mgr-join-%s-%s", env.ClusterID, member.Address),
			"action":        "join",
			"group_name":    groupName,
			"local_address": fmt.Sprintf("%s:%d", member.Address, localPort),
			"group_seeds":   seeds,
			"bootstrap":     false,
		}
		if _, err := p.agentCaller(ctx, member.Address, member.AgentPort, "/api/v1/mgr/setup", joinPayload); err != nil {
			return nil, fmt.Errorf("MGR join on %s: %w", member.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("MGR cluster assembled: %d nodes in group %s", len(env.Nodes), groupName),
	}, nil
}

func (p *MGRAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *MGRAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
