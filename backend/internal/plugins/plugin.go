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
	Join(ctx context.Context, env PluginEnv, newNode PluginNode) error
	Leave(ctx context.Context, env PluginEnv, node PluginNode) error
}

type PluginEnv struct {
	ClusterID   string
	Nodes       []PluginNode
	Credentials CredentialSet
	Custom      map[string]interface{}
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

type PluginParams struct {
	DeployMode    string            `json:"deploy_mode,omitempty"`
	Flavor        string            `json:"flavor,omitempty"`
	Version       string            `json:"version,omitempty"`
	ServerID      int               `json:"server_id,omitempty"`
	GTIDDomainID  int               `json:"gtid_domain_id,omitempty"`
	VIP           string            `json:"vip,omitempty"`
	VIPInterface  string            `json:"vip_interface,omitempty"`
	ProxyHost     string            `json:"proxy_host,omitempty"`
	ProxyPort     int               `json:"proxy_port,omitempty"`
	GroupName     string            `json:"group_name,omitempty"`
	LocalAddress  string            `json:"local_address,omitempty"`
	GroupSeeds    []string          `json:"group_seeds,omitempty"`
	Bootstrap     bool              `json:"bootstrap,omitempty"`
	Custom        map[string]string `json:"custom,omitempty"`
}

func (p *PluginParams) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	if p.DeployMode != "" { m["deploy_mode"] = p.DeployMode }
	if p.Flavor != "" { m["flavor"] = p.Flavor }
	if p.Version != "" { m["version"] = p.Version }
	if p.ServerID != 0 { m["server_id"] = p.ServerID }
	if p.GTIDDomainID != 0 { m["gtid_domain_id"] = p.GTIDDomainID }
	if p.VIP != "" { m["vip"] = p.VIP }
	if p.VIPInterface != "" { m["vip_interface"] = p.VIPInterface }
	if p.ProxyHost != "" { m["proxy_host"] = p.ProxyHost }
	if p.ProxyPort != 0 { m["proxy_port"] = p.ProxyPort }
	if p.GroupName != "" { m["group_name"] = p.GroupName }
	if p.LocalAddress != "" { m["local_address"] = p.LocalAddress }
	if len(p.GroupSeeds) > 0 { m["group_seeds"] = p.GroupSeeds }
	if p.Bootstrap { m["bootstrap"] = p.Bootstrap }
	if len(p.Custom) > 0 {
		for k, v := range p.Custom { m[k] = v }
	}
	return m
}

type DefaultPluginMethods struct{}

func (d *DefaultPluginMethods) Join(_ context.Context, _ PluginEnv, _ PluginNode) error {
	return nil
}

func (d *DefaultPluginMethods) Leave(_ context.Context, _ PluginEnv, _ PluginNode) error {
	return nil
}
