package arch

import (
	"context"
	"fmt"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type PXCGaleraAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func (p *PXCGaleraAddonPlugin) Join(ctx context.Context, env plugins.PluginEnv, newNode plugins.PluginNode) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	wsrepPort := 4567
	if v, ok := env.Custom["wsrep_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			wsrepPort = port
		}
	}
	bootstrap := env.Nodes[0]
	payload := map[string]interface{}{
		"task_id":            fmt.Sprintf("pxc-join-%s-%s", env.ClusterID, newNode.Address),
		"action":             "join",
		"wsrep_cluster_addr": fmt.Sprintf("gcomm://%s:%d", bootstrap.Address, wsrepPort),
		"node_address":       newNode.Address,
	}
	_, err := p.agentCaller(ctx, newNode.Address, newNode.AgentPort, "/api/v1/pxc/setup", payload)
	return err
}

func (p *PXCGaleraAddonPlugin) Leave(_ context.Context, _ plugins.PluginEnv, _ plugins.PluginNode) error {
	return nil
}

func NewPXCGaleraAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *PXCGaleraAddonPlugin {
	return &PXCGaleraAddonPlugin{agentCaller: agentCaller}
}

func (p *PXCGaleraAddonPlugin) Name() string         { return "pxc-galera-addon" }
func (p *PXCGaleraAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeArch }
func (p *PXCGaleraAddonPlugin) Version() string       { return "1.0.0" }

func (p *PXCGaleraAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 1 {
		return fmt.Errorf("PXC addon requires at least 1 node")
	}
	return nil
}

func (p *PXCGaleraAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}
	wsrepPort := 4567
	if v, ok := env.Custom["wsrep_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			wsrepPort = port
		}
	}

	bootstrap := env.Nodes[0]
	bootstrapPayload := map[string]interface{}{
		"task_id":           fmt.Sprintf("pxc-bootstrap-%s", env.ClusterID),
		"action":            "bootstrap",
		"wsrep_cluster_addr": "gcomm://",
		"node_address":      bootstrap.Address,
	}
	if _, err := p.agentCaller(ctx, bootstrap.Address, bootstrap.AgentPort, "/api/v1/pxc/setup", bootstrapPayload); err != nil {
		return nil, fmt.Errorf("PXC bootstrap on %s: %w", bootstrap.Address, err)
	}

	for _, node := range env.Nodes[1:] {
		joinPayload := map[string]interface{}{
			"task_id":           fmt.Sprintf("pxc-join-%s-%s", env.ClusterID, node.Address),
			"action":            "join",
			"wsrep_cluster_addr": fmt.Sprintf("gcomm://%s:%d", bootstrap.Address, wsrepPort),
			"node_address":      node.Address,
		}
		if _, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/pxc/setup", joinPayload); err != nil {
			return nil, fmt.Errorf("PXC join on %s: %w", node.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("PXC Galera cluster assembled: %d nodes", len(env.Nodes)),
	}, nil
}

func (p *PXCGaleraAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *PXCGaleraAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
