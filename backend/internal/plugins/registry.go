package plugins

import (
	"fmt"
	"sync"
)

// DescribedPlugin 由动态插件实现, 提供目录展示用的扩展元信息 (endpoint/说明).
// 内置插件不实现它, Registry 用 Name/Type/Version 拼默认信息.
type DescribedPlugin interface {
	Info() PluginInfo
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	// metadataOnly: 仅登记展示信息、未实现 plugins.Plugin 执行接口的插件
	// (kernel 插件是独立 KernelPlugin 接口, 尚无执行链路消费方).
	metadataOnly map[string]PluginInfo
	// dynamic: 运行期经 API 接入的插件. 只有这些允许 Unregister, 内置插件不可注销.
	dynamic map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{
		plugins:      make(map[string]Plugin),
		metadataOnly: make(map[string]PluginInfo),
		dynamic:      make(map[string]bool),
	}
}

// PluginInfo: 插件目录展示用的元信息 (不承载执行能力).
type PluginInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Source      string `json:"source,omitempty"` // builtin | dynamic
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description,omitempty"`
}

// RegisterMetadata 登记仅用于目录展示的插件元信息 (不可执行).
func (r *Registry) RegisterMetadata(info PluginInfo) error {
	if info.Name == "" {
		return fmt.Errorf("plugin name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %q already registered", info.Name)
	}
	info.Source = "builtin"
	r.metadataOnly[info.Name] = info
	return nil
}

func (r *Registry) Register(p Plugin) error {
	return r.register(p, false)
}

// RegisterDynamic 接入运行期动态插件 (可被 Unregister).
func (r *Registry) RegisterDynamic(p Plugin) error {
	return r.register(p, true)
}

func (r *Registry) register(p Plugin, dynamic bool) error {
	if p == nil {
		return fmt.Errorf("plugin is nil")
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	if _, exists := r.metadataOnly[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	r.plugins[name] = p
	if dynamic {
		r.dynamic[name] = true
	}
	return nil
}

// Unregister 取消注册动态插件. 内置插件受保护, 避免误操作让部署链路失去插件.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.dynamic[name] {
		if _, ok := r.plugins[name]; ok {
			return fmt.Errorf("plugin %q is builtin and cannot be unregistered", name)
		}
		return fmt.Errorf("plugin %q not found", name)
	}
	delete(r.plugins, name)
	delete(r.dynamic, name)
	return nil
}

func (r *Registry) Get(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	return p, nil
}

// infoOf 必须在持有 r.mu 读锁时调用.
func (r *Registry) infoOf(p Plugin) PluginInfo {
	source := "builtin"
	if r.dynamic[p.Name()] {
		source = "dynamic"
	}
	if described, ok := p.(DescribedPlugin); ok {
		info := described.Info()
		info.Source = source
		return info
	}
	return PluginInfo{Name: p.Name(), Type: string(p.Type()), Version: p.Version(), Source: source}
}

// GetInfo: 按名称查插件元信息 (完整插件与仅元数据登记均覆盖).
func (r *Registry) GetInfo(name string) (PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.plugins[name]; ok {
		return r.infoOf(p), true
	}
	info, ok := r.metadataOnly[name]
	return info, ok
}

func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out
}

// ListAllInfo: 全部插件的元信息 (完整插件 + 仅元数据登记), 按名称排序供目录展示.
func (r *Registry) ListAllInfo() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PluginInfo, 0, len(r.plugins)+len(r.metadataOnly))
	for _, p := range r.plugins {
		out = append(out, r.infoOf(p))
	}
	for _, info := range r.metadataOnly {
		out = append(out, info)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Name < out[j-1].Name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (r *Registry) ListByType(t PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Plugin
	for _, p := range r.plugins {
		if p.Type() == t {
			out = append(out, p)
		}
	}
	return out
}
