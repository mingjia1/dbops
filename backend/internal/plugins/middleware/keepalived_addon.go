package middleware

import (
	"context"
	"fmt"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
)

type KeepalivedAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
	plugins.DefaultPluginMethods
}

func NewKeepalivedAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *KeepalivedAddonPlugin {
	return &KeepalivedAddonPlugin{agentCaller: agentCaller}
}

func (p *KeepalivedAddonPlugin) Name() string         { return "keepalived-addon" }
func (p *KeepalivedAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeMiddleware }
func (p *KeepalivedAddonPlugin) Version() string       { return "1.0.0" }

func (p *KeepalivedAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 2 {
		return fmt.Errorf("keepalived requires at least 2 nodes")
	}
	return nil
}

func (p *KeepalivedAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}

	vip, _ := params["vip"].(string)
	vipInterface, _ := params["vip_interface"].(string)
	if vip == "" {
		vip = "10.0.0.100"
	}
	if vipInterface == "" {
		vipInterface = "eth0"
	}

	for i, node := range env.Nodes {
		priority := 100 - i
		role := "BACKUP"
		if i == 0 {
			role = "MASTER"
		}

		payload := map[string]interface{}{
			"task_id":      fmt.Sprintf("keepalived-%s-%d", env.ClusterID, i),
			"vip":          vip,
			"vip_interface": vipInterface,
			"priority":     priority,
			"role":         role,
			"mysql_port":   node.MySQLPort,
		}
		if _, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/api/v1/keepalived/setup", payload); err != nil {
			return nil, fmt.Errorf("keepalived setup on %s: %w", node.Address, err)
		}
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("Keepalived deployed on %d nodes with VIP %s", len(env.Nodes), vip),
	}, nil
}

func (p *KeepalivedAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *KeepalivedAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}
