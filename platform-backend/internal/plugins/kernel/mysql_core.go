package kernel

import (
	"context"
	"fmt"
	"log"
)

type MySQLCorePlugin struct {
	agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)
}

func NewMySQLCorePlugin(agentCaller func(ctx context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error)) *MySQLCorePlugin {
	return &MySQLCorePlugin{agentCaller: agentCaller}
}

func (p *MySQLCorePlugin) Name() string    { return "mysql-core" }
func (p *MySQLCorePlugin) Type() string    { return "kernel" }
func (p *MySQLCorePlugin) Version() string { return "1.0.0" }

type KernelPlugin interface {
	Name() string
	Type() string
	Version() string
	Prepare(ctx context.Context, env KernelEnv) error
	Execute(ctx context.Context, env KernelEnv, params map[string]interface{}) (*KernelResult, error)
	Rollback(ctx context.Context, env KernelEnv) error
	Teardown(ctx context.Context, env KernelEnv) error
	Flavor() string
}

type KernelEnv struct {
	ClusterID string
	Node      KernelNode
	Creds     KernelCredentials
}

type KernelNode struct {
	HostID    string
	Address   string
	AgentPort int
	MySQLPort int
	Role      string
	DataDir   string
	Basedir   string
	ServerID  int
}

type KernelCredentials struct {
	RootPassword    string
	ReplUser        string
	ReplPassword    string
	MonitorUser     string
	MonitorPassword string
}

type KernelResult struct {
	Success  bool                   `json:"success"`
	Message  string                 `json:"message"`
	Basedir  string                 `json:"basedir"`
	Datadir  string                 `json:"datadir"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
}

func (p *MySQLCorePlugin) Flavor() string { return "mysql" }

func (p *MySQLCorePlugin) Prepare(ctx context.Context, env KernelEnv) error {
	if env.Node.Address == "" {
		return fmt.Errorf("node address is empty")
	}
	if env.Node.AgentPort == 0 {
		return fmt.Errorf("node agent port is zero")
	}
	if env.Node.MySQLPort == 0 {
		env.Node.MySQLPort = 3306
	}
	if env.Node.DataDir == "" {
		env.Node.DataDir = fmt.Sprintf("/data/mysql_%d", env.Node.MySQLPort)
	}
	if env.Node.Basedir == "" {
		env.Node.Basedir = fmt.Sprintf("/opt/mysql-%d", env.Node.MySQLPort)
	}
	return nil
}

func (p *MySQLCorePlugin) Execute(ctx context.Context, env KernelEnv, params map[string]interface{}) (*KernelResult, error) {
	if p.agentCaller == nil {
		return nil, fmt.Errorf("agent caller not configured")
	}

	payload := map[string]interface{}{
		"task_id":       fmt.Sprintf("deploy-%s-%d", env.ClusterID, env.Node.MySQLPort),
		"deploy_mode":   "single",
		"host":          env.Node.Address,
		"port":          env.Node.MySQLPort,
		"data_dir":      env.Node.DataDir,
		"basedir":       env.Node.Basedir,
		"mysql_user":    "root",
		"mysql_pass":    env.Creds.RootPassword,
		"repl_user":     env.Creds.ReplUser,
		"repl_pass":     env.Creds.ReplPassword,
		"monitor_user":  env.Creds.MonitorUser,
		"monitor_pass":  env.Creds.MonitorPassword,
		"server_id":     env.Node.ServerID,
		"flavor":        "mysql",
	}
	if env.Node.Role != "" {
		payload["role"] = env.Node.Role
	}

	log.Printf("INFO: mysql-core executing on %s:%d", env.Node.Address, env.Node.AgentPort)

	resp, err := p.agentCaller(ctx, env.Node.Address, env.Node.AgentPort, "/api/v1/deploy/init", payload)
	if err != nil {
		return nil, fmt.Errorf("agent deploy init: %w", err)
	}

	return &KernelResult{
		Success: true,
		Message: fmt.Sprintf("MySQL instance deployed on %s:%d", env.Node.Address, env.Node.MySQLPort),
		Basedir: env.Node.Basedir,
		Datadir: env.Node.DataDir,
		Data:    resp,
	}, nil
}

func (p *MySQLCorePlugin) Rollback(ctx context.Context, env KernelEnv) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"port":     env.Node.MySQLPort,
		"data_dir": env.Node.DataDir,
	}
	_, err := p.agentCaller(ctx, env.Node.Address, env.Node.AgentPort, "/api/v1/deploy/rollback", payload)
	return err
}

func (p *MySQLCorePlugin) Teardown(ctx context.Context, env KernelEnv) error {
	if p.agentCaller == nil {
		return fmt.Errorf("agent caller not configured")
	}
	payload := map[string]interface{}{
		"port":     env.Node.MySQLPort,
		"data_dir": env.Node.DataDir,
		"basedir":  env.Node.Basedir,
	}
	_, err := p.agentCaller(ctx, env.Node.Address, env.Node.AgentPort, "/api/v1/deploy/teardown", payload)
	return err
}
