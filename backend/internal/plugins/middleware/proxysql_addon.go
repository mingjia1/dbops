package middleware

import (
	"context"
	"fmt"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type ProxySQLAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
	plugins.DefaultPluginMethods
}

func NewProxySQLAddonPlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *ProxySQLAddonPlugin {
	return &ProxySQLAddonPlugin{agentCaller: agentCaller}
}

func (p *ProxySQLAddonPlugin) Name() string             { return "proxysql-addon" }
func (p *ProxySQLAddonPlugin) Type() plugins.PluginType { return plugins.PluginTypeMiddleware }
func (p *ProxySQLAddonPlugin) Version() string          { return "1.0.0" }

func (p *ProxySQLAddonPlugin) Prepare(_ context.Context, env plugins.PluginEnv) error {
	if len(env.Nodes) < 1 {
		return fmt.Errorf("proxysql requires at least 1 backend node")
	}
	return nil
}

func (p *ProxySQLAddonPlugin) Execute(ctx context.Context, env plugins.PluginEnv, params map[string]interface{}) (*plugins.PluginResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}

	proxyHost, _ := params["proxy_host"].(string)
	proxyPort := intParam(params, "proxy_port")
	proxyAgentPort := intParam(params, "agent_port")
	if proxyAgentPort == 0 {
		proxyAgentPort = intParam(params, "proxy_agent_port")
	}
	if proxyHost == "" {
		return nil, fmt.Errorf("proxy_host is required")
	}
	if proxyPort == 0 {
		proxyPort = 6033
	}
	if proxyAgentPort == 0 {
		proxyAgentPort = 9090
	}

	var backends []map[string]interface{}
	for _, n := range env.Nodes {
		backends = append(backends, map[string]interface{}{
			"host": n.Address,
			"port": n.MySQLPort,
		})
	}

	payload := map[string]interface{}{
		"task_id": fmt.Sprintf("proxysql-%s", env.ClusterID),
		"config": map[string]interface{}{
			"proxy_host": proxyHost,
			"proxy_port": proxyPort,
			"backends":   backends,
			"admin_user": env.Credentials.RootUser,
			"admin_pass": env.Credentials.RootPassword,
		},
	}

	resp, err := p.agentCaller(ctx, proxyHost, proxyAgentPort, "/agent/tasks/proxysql-setup", payload)
	if err != nil {
		return nil, fmt.Errorf("proxysql setup: %w", err)
	}
	if err := ensureAgentTaskSucceeded("proxysql", proxyHost, resp); err != nil {
		return nil, err
	}

	return &plugins.PluginResult{
		Success: true,
		Message: fmt.Sprintf("ProxySQL configured with %d backends", len(backends)),
	}, nil
}

func (p *ProxySQLAddonPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func (p *ProxySQLAddonPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error {
	return nil
}

func intParam(params map[string]interface{}, key string) int {
	if params == nil {
		return 0
	}
	switch v := params[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var out int
		if _, err := fmt.Sscanf(v, "%d", &out); err == nil {
			return out
		}
	}
	return 0
}
