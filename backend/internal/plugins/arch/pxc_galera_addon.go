package arch

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
)

type PXCGaleraAddonPlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func stringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// setupPayload builds the /api/v1/pxc/setup request body. The agent maps
// action=join/bootstrap onto a full DeployPXC run and action=leave onto a
// non-destructive stop, so it needs port/datadir/credentials here.
func (p *PXCGaleraAddonPlugin) setupPayload(env plugins.PluginEnv, node plugins.PluginNode, action, wsrepAddr string) map[string]interface{} {
	payload := map[string]interface{}{
		"task_id":      fmt.Sprintf("pxc-%s-%s-%s", action, env.ClusterID, node.Address),
		"action":       action,
		"node_address": node.Address,
	}
	if wsrepAddr != "" {
		payload["wsrep_cluster_addr"] = wsrepAddr
	}
	if node.MySQLPort > 0 {
		payload["mysql_port"] = node.MySQLPort
	}
	if node.DataDir != "" {
		payload["data_dir"] = node.DataDir
	}
	if node.Basedir != "" {
		payload["basedir"] = node.Basedir
	}
	if env.Credentials.RootUser != "" {
		payload["mysql_user"] = env.Credentials.RootUser
	}
	if env.Credentials.RootPassword != "" {
		payload["mysql_password"] = env.Credentials.RootPassword
	}
	if env.Credentials.ReplUser != "" {
		payload["replicate_user"] = env.Credentials.ReplUser
		payload["replicate_pass"] = env.Credentials.ReplPassword
	}
	if name := stringFromMap(env.Custom, "cluster_name"); name != "" {
		payload["cluster_name"] = name
	} else if env.ClusterID != "" {
		payload["cluster_name"] = env.ClusterID
	}
	if sst := stringFromMap(env.Custom, "sst_method"); sst != "" {
		payload["sst_method"] = sst
	}
	return payload
}

func (p *PXCGaleraAddonPlugin) call(ctx context.Context, host string, agentPort int, payload map[string]interface{}) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	_, err := p.agentCaller(ctx, host, agentPort, "/api/v1/pxc/setup", payload)
	return err
}

func (p *PXCGaleraAddonPlugin) Join(ctx context.Context, env plugins.PluginEnv, newNode plugins.PluginNode) error {
	wsrepPort := 4567
	if v, ok := env.Custom["wsrep_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			wsrepPort = port
		}
	}
	bootstrap := env.Nodes[0]
	addr := fmt.Sprintf("gcomm://%s:%d", bootstrap.Address, wsrepPort)
	return p.call(ctx, newNode.Address, newNode.AgentPort, p.setupPayload(env, newNode, "join", addr))
}

func (p *PXCGaleraAddonPlugin) Leave(ctx context.Context, env plugins.PluginEnv, node plugins.PluginNode) error {
	err := p.call(ctx, node.Address, node.AgentPort, p.setupPayload(env, node, "leave", ""))
	if err != nil {
		fmt.Printf("WARN: PXC leave failed for %s: %v — remaining Galera nodes may need manual cleanup\n", node.Address, err)
	}
	return err
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
	wsrepPort := 4567
	if v, ok := env.Custom["wsrep_port"]; ok {
		if port, ok := v.(int); ok && port > 0 {
			wsrepPort = port
		}
	}

	// Find bootstrap node by role (not by position) to handle any node order.
	var bootstrap *plugins.PluginNode
	for i, n := range env.Nodes {
		if n.Role == "bootstrap" {
			node := env.Nodes[i]
			bootstrap = &node
			break
		}
	}
	if bootstrap == nil {
		// Fallback: first node
		node := env.Nodes[0]
		bootstrap = &node
	}
	bootstrapPayload := p.setupPayload(env, *bootstrap, "bootstrap", "")
	if err := p.call(ctx, bootstrap.Address, bootstrap.AgentPort, bootstrapPayload); err != nil {
		return nil, fmt.Errorf("PXC bootstrap on %s: %w", bootstrap.Address, err)
	}

	for _, node := range env.Nodes[1:] {
		addr := fmt.Sprintf("gcomm://%s:%d", bootstrap.Address, wsrepPort)
		if err := p.call(ctx, node.Address, node.AgentPort, p.setupPayload(env, node, "join", addr)); err != nil {
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

func (p *PXCGaleraAddonPlugin) Teardown(ctx context.Context, env plugins.PluginEnv) error {
	// Best-effort: ask each node to leave the Galera cluster (stop, keep data).
	var errs []error
	for _, node := range env.Nodes {
		if err := p.call(ctx, node.Address, node.AgentPort, p.setupPayload(env, node, "leave", "")); err != nil {
			errs = append(errs, fmt.Errorf("leave %s: %w", node.Address, err))
		}
	}
	return errors.Join(errs...)
}
