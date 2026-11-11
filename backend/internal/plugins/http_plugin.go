package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultHTTPPluginTimeout = 30 * time.Second
	maxHTTPPluginTimeout     = 10 * time.Minute
	maxHTTPPluginResponse    = 1 << 20 // 1 MiB
)

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

// HTTPPluginConfig 描述一个外部 HTTP 插件. Go 无法在运行时安全热加载 .so
// (Linux-only 且要求构建环境完全一致), 所以动态插件走 HTTP: 外部服务实现
// 约定协议即可接入/注销, 无需重新编译平台.
type HTTPPluginConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // kernel | arch | middleware | custom
	Version     string `json:"version"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description,omitempty"`
	Token       string `json:"token,omitempty"`
	TimeoutMS   int    `json:"timeout_ms,omitempty"`
}

func (c HTTPPluginConfig) Validate() error {
	if !pluginNamePattern.MatchString(c.Name) {
		return fmt.Errorf("name must match ^[a-z0-9][a-z0-9._-]{1,63}$")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("endpoint must be an absolute http(s) URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("endpoint must not contain a query string or fragment")
	}
	if c.TimeoutMS < 0 || time.Duration(c.TimeoutMS)*time.Millisecond > maxHTTPPluginTimeout {
		return fmt.Errorf("timeout_ms must be between 0 and %d", maxHTTPPluginTimeout/time.Millisecond)
	}
	return nil
}

// HTTPPlugin 把 Plugin 的 7 个方法映射为 POST <endpoint>/<action>.
type HTTPPlugin struct {
	cfg    HTTPPluginConfig
	client *http.Client
}

func NewHTTPPlugin(cfg HTTPPluginConfig) *HTTPPlugin {
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Type == "" {
		cfg.Type = string(PluginTypeMiddleware)
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultHTTPPluginTimeout
	}
	return &HTTPPlugin{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *HTTPPlugin) Name() string                    { return p.cfg.Name }
func (p *HTTPPlugin) Type() PluginType                { return PluginType(p.cfg.Type) }
func (p *HTTPPlugin) Version() string                 { return p.cfg.Version }
func (p *HTTPPlugin) Config() HTTPPluginConfig        { return p.cfg }
func (p *HTTPPlugin) Info() PluginInfo {
	return PluginInfo{
		Name:        p.cfg.Name,
		Type:        p.cfg.Type,
		Version:     p.cfg.Version,
		Endpoint:   p.cfg.Endpoint,
		Description: p.cfg.Description,
		Source:      "dynamic",
	}
}

func (p *HTTPPlugin) Prepare(ctx context.Context, env PluginEnv) error {
	return p.callErr(ctx, "prepare", env, nil, nil)
}

func (p *HTTPPlugin) Execute(ctx context.Context, env PluginEnv, params map[string]interface{}) (*PluginResult, error) {
	res, err := p.call(ctx, "execute", env, params, nil)
	if err != nil {
		return nil, err
	}
	if !res.Success {
		return nil, fmt.Errorf("plugin %s execute failed: %s", p.cfg.Name, res.Message)
	}
	return res, nil
}

func (p *HTTPPlugin) Rollback(ctx context.Context, env PluginEnv) error {
	return p.callErr(ctx, "rollback", env, nil, nil)
}

func (p *HTTPPlugin) Teardown(ctx context.Context, env PluginEnv) error {
	return p.callErr(ctx, "teardown", env, nil, nil)
}

func (p *HTTPPlugin) Join(ctx context.Context, env PluginEnv, newNode PluginNode) error {
	return p.callErr(ctx, "join", env, nil, &newNode)
}

func (p *HTTPPlugin) Leave(ctx context.Context, env PluginEnv, node PluginNode) error {
	return p.callErr(ctx, "leave", env, nil, &node)
}

// Ping 供「测试连接」使用, 不触碰真实环境数据.
func (p *HTTPPlugin) Ping(ctx context.Context) error {
	return p.callErr(ctx, "ping", PluginEnv{}, nil, nil)
}

type pluginHTTPRequest struct {
	Action string                 `json:"action"`
	Plugin string                 `json:"plugin"`
	Env    PluginEnv              `json:"env"`
	Params map[string]interface{} `json:"params,omitempty"`
	Node   *PluginNode            `json:"node,omitempty"`
}

func (p *HTTPPlugin) call(ctx context.Context, action string, env PluginEnv, params map[string]interface{}, node *PluginNode) (*PluginResult, error) {
	body, err := json.Marshal(pluginHTTPRequest{
		Action: action,
		Plugin: p.cfg.Name,
		Env:    env,
		Params: params,
		Node:   node,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal plugin request: %w", err)
	}

	target := strings.TrimRight(p.cfg.Endpoint, "/") + "/" + action
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build plugin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DBOPS-Action", action)
	if p.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %s %s: %w", p.cfg.Name, action, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPPluginResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read plugin response: %w", err)
	}
	if len(raw) > maxHTTPPluginResponse {
		return nil, fmt.Errorf("plugin %s %s: response exceeds 1 MiB", p.cfg.Name, action)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("plugin %s %s: HTTP %d: %s", p.cfg.Name, action, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return decodePluginResponse(raw, p.cfg.Name, action)
}

func (p *HTTPPlugin) callErr(ctx context.Context, action string, env PluginEnv, params map[string]interface{}, node *PluginNode) error {
	res, err := p.call(ctx, action, env, params, node)
	if err != nil {
		return err
	}
	if !res.Success {
		return fmt.Errorf("plugin %s %s failed: %s", p.cfg.Name, action, res.Message)
	}
	return nil
}

// decodePluginResponse: success 缺省视为成功 (2xx + 空 body 即成功),
// 显式 "success": false 才判失败, 这样第三方最小实现只需返回 HTTP 200.
func decodePluginResponse(raw []byte, pluginName, action string) (*PluginResult, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &PluginResult{Success: true}, nil
	}
	var wire struct {
		Success  *bool                  `json:"success"`
		Message  string                 `json:"message"`
		Data     map[string]interface{} `json:"data"`
		Warnings []string               `json:"warnings"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("plugin %s %s: invalid JSON response: %w", pluginName, action, err)
	}
	return &PluginResult{
		Success:  wire.Success == nil || *wire.Success,
		Message:  wire.Message,
		Data:     wire.Data,
		Warnings: wire.Warnings,
	}, nil
}
