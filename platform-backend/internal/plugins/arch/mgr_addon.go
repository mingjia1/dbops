package arch

import (
	"context"
	"fmt"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type MGRAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
	plugins.DefaultPluginMethods
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
		seeds = append(seeds, fmt.Sprintf("%s:%d", n.Address, 33061))
	}

	bootstrapPayload := map[string]interface{}{
		"task_id":       fmt.Sprintf("mgr-bootstrap-%s", env.ClusterID),
		"action":        "bootstrap",
		"group_name":    env.ClusterID,
		"local_address": fmt.Sprintf("%s:33061", primary.Address),
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
			"group_name":    env.ClusterID,
			"local_address": fmt.Sprintf("%s:33061", member.Address),
			"group_seeds":   seeds,
			"bootstrap":     false,
		}
		if _, err := p.agentCaller(ctx, member.Address, member.AgentPort, "/api/v1/mgr/setup", joinPayload); err != nil {
			return nil, fmt.Errorf("MGR join on %s: %w", member.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("MGR cluster assembled: %d nodes in group %s", len(env.Nodes), env.ClusterID),
	}, nil
}

func (p *MGRAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *MGRAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
