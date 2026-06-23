package kernel

import (
	"context"
	"fmt"
)

type MariaDBCorePlugin struct {
	base *MySQLCorePlugin
}

func NewMariaDBCorePlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *MariaDBCorePlugin {
	return &MariaDBCorePlugin{base: NewMySQLCorePlugin(agentCaller)}
}

func (p *MariaDBCorePlugin) Name() string    { return "mariadb-core" }
func (p *MariaDBCorePlugin) Type() string    { return "kernel" }
func (p *MariaDBCorePlugin) Version() string { return "1.0.0" }
func (p *MariaDBCorePlugin) Flavor() string  { return "mariadb" }

func (p *MariaDBCorePlugin) Prepare(ctx context.Context, env KernelEnv) error {
	return p.base.Prepare(ctx, env)
}

func (p *MariaDBCorePlugin) Execute(ctx context.Context, env KernelEnv, params map[string]interface{}) (*KernelResult, error) {
	if p.base.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"task_id":        fmt.Sprintf("deploy-%s-%d", env.ClusterID, env.Node.MySQLPort),
		"deploy_mode":    "single",
		"host":           env.Node.Address,
		"port":           env.Node.MySQLPort,
		"data_dir":       env.Node.DataDir,
		"basedir":        env.Node.Basedir,
		"mysql_user":     "root",
		"mysql_pass":     env.Creds.RootPassword,
		"repl_user":      env.Creds.ReplUser,
		"repl_pass":      env.Creds.ReplPassword,
		"monitor_user":   env.Creds.MonitorUser,
		"monitor_pass":   env.Creds.MonitorPassword,
		"server_id":      env.Node.ServerID,
		"flavor":         "mariadb",
		"gtid_domain_id": env.Node.ServerID,
	}
	if env.Node.Role != "" {
		payload["role"] = env.Node.Role
	}

	resp, err := p.base.agentCaller(ctx, env.Node.Address, env.Node.AgentPort, "/api/v1/deploy/init", payload)
	if err != nil {
		return nil, fmt.Errorf("agent deploy init: %w", err)
	}

	return &KernelResult{
		Success: true,
		Message: fmt.Sprintf("MariaDB instance deployed on %s:%d", env.Node.Address, env.Node.MySQLPort),
		Basedir: env.Node.Basedir,
		Datadir: env.Node.DataDir,
		Data:    resp,
	}, nil
}

func (p *MariaDBCorePlugin) Rollback(ctx context.Context, env KernelEnv) error {
	return p.base.Rollback(ctx, env)
}

func (p *MariaDBCorePlugin) Teardown(ctx context.Context, env KernelEnv) error {
	return p.base.Teardown(ctx, env)
}
