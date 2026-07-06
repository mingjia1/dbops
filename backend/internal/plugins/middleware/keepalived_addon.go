package middleware

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type KeepalivedAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
	plugins.DefaultPluginMethods
}

func NewKeepalivedAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *KeepalivedAddonPlugin {
	return &KeepalivedAddonPlugin{agentCaller: agentCaller}
}

func (p *KeepalivedAddonPlugin) Name() string             { return "keepalived-addon" }
func (p *KeepalivedAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeMiddleware }
func (p *KeepalivedAddonPlugin) Version() string          { return "1.0.0" }

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
	vrid := intParam(params, "virtual_router_id")
	if vrid == 0 {
		vrid = stableKeepalivedVRID(env.ClusterID)
	}
	authPass, _ := params["auth_pass"].(string)
	if authPass == "" {
		authPass = stableKeepalivedAuthPass(env.ClusterID, vrid)
	}

	for i, node := range env.Nodes {
		priority := 100 - i
		role := "BACKUP"
		if i == 0 {
			role = "MASTER"
		}

		payload := map[string]interface{}{
			"task_id": fmt.Sprintf("keepalived-%s-%d", env.ClusterID, i),
			"config": map[string]interface{}{
				"vip":               vip,
				"vip_interface":     vipInterface,
				"priority":          priority,
				"role":              role,
				"mysql_port":        node.MySQLPort,
				"mysql_user":        env.Credentials.RootUser,
				"mysql_pass":        env.Credentials.RootPassword,
				"virtual_router_id": vrid,
				"auth_pass":         authPass,
			},
		}
		resp, err := p.agentCaller(ctx, node.Address, node.AgentPort, "/agent/tasks/keepalived-setup", payload)
		if err != nil {
			return nil, fmt.Errorf("keepalived setup on %s: %w", node.Address, err)
		}
		if err := ensureAgentTaskSucceeded("keepalived", node.Address, resp); err != nil {
			return nil, err
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

func stableKeepalivedVRID(clusterID string) int {
	if clusterID == "" {
		return 51
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(clusterID))
	return int(h.Sum32()%255) + 1
}

func stableKeepalivedAuthPass(clusterID string, vrid int) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(clusterID))
	_, _ = h.Write([]byte(fmt.Sprintf(":%d", vrid)))
	return fmt.Sprintf("k%07x", h.Sum32()&0x0fffffff)[:8]
}
