package arch

import (
	"context"
	"errors"
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
	// H2: Derive per-node local_port from base + node index.
	// env.Nodes must include all existing cluster nodes so that
	// each new node gets a unique local_port.
	basePort := 33061
	if v, ok := env.Custom["local_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			basePort = port
		}
	}
	// The new node is the last element in env.Nodes; its index is len-1.
	localPort := basePort + len(env.Nodes) - 1
	if localPort <= 0 {
		localPort = basePort
	}
	groupName := env.ClusterID
	if v, ok := env.Custom["group_name"]; ok {
		if name, ok := v.(string); ok && name != "" {
			groupName = name
		}
	}
	// Build seeds with each node's correct local_port.
	seeds := make([]string, 0, len(env.Nodes))
	for i, n := range env.Nodes {
		seeds = append(seeds, fmt.Sprintf("%s:%d", n.Address, basePort+i))
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

func (p *MGRAddonPlugin) Teardown(ctx context.Context, env plugins.PluginEnv) error {
	if p.agentCaller == nil {
		return nil
	}
	// Best-effort: ask each non-primary node to leave the group.
	// Primary/bootstrap cannot be removed via leave while it's the last primary.
	var errs []error
	for _, node := range env.Nodes {
		if node.Role == "primary" || node.Role == "bootstrap" {
			continue
		}
		payload := map[string]interface{}{
			"task_id": fmt.Sprintf("mgr-teardown-%s-%s", env.ClusterID, node.Address),
			"action":  "leave",
		}
		if _, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/mgr/setup", payload); err != nil {
			errs = append(errs, fmt.Errorf("leave %s: %w", node.Address, err))
		}
	}
	return errors.Join(errs...)
}
