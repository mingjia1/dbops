package plugins

import "context"

type PluginType string

const (
	PluginTypeKernel     PluginType = "kernel"
	PluginTypeArch       PluginType = "arch"
	PluginTypeMiddleware PluginType = "middleware"
)

type Plugin interface {
	Name() string
	Type() PluginType
	Version() string
	Prepare(ctx context.Context, env PluginEnv) error
	Execute(ctx context.Context, env PluginEnv, params map[string]interface{}) (*PluginResult, error)
	Rollback(ctx context.Context, env PluginEnv) error
	Teardown(ctx context.Context, env PluginEnv) error
}

type PluginEnv struct {
	ClusterID   string
	Nodes       []PluginNode
	Credentials CredentialSet
}

type PluginNode struct {
	HostID    string
	Address   string
	AgentPort int
	MySQLPort int
	Role      string
	DataDir   string
	Basedir   string
}

type CredentialSet struct {
	RootUser        string
	RootPassword    string
	ReplUser        string
	ReplPassword    string
	MonitorUser     string
	MonitorPassword string
}

type PluginResult struct {
	Success  bool                   `json:"success"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
}
