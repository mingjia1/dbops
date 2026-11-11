package plugins

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	// metadataOnly: 仅登记展示信息、未实现 plugins.Plugin 执行接口的插件
	// (kernel 插件是独立 KernelPlugin 接口, 尚无执行链路消费方).
	metadataOnly map[string]PluginInfo
}

func NewRegistry() *Registry {
	return &Registry{
		plugins:      make(map[string]Plugin),
		metadataOnly: make(map[string]PluginInfo),
	}
}

// RegisterMetadata 登记仅用于目录展示的插件元信息 (不可执行).
// PluginInfo: 插件目录展示用的元信息 (不承载执行能力).
type PluginInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

func (r *Registry) RegisterMetadata(info PluginInfo) error {
	if info.Name == "" {
		return fmt.Errorf("plugin name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %q already registered", info.Name)
	}
	r.metadataOnly[info.Name] = info
	return nil
}

func (r *Registry) Register(p Plugin) error {
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
	r.plugins[name] = p
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

// GetInfo: 按名称查插件元信息 (完整插件与仅元数据登记均覆盖).
func (r *Registry) GetInfo(name string) (PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.plugins[name]; ok {
		return PluginInfo{Name: p.Name(), Type: string(p.Type()), Version: p.Version()}, true
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
		out = append(out, PluginInfo{Name: p.Name(), Type: string(p.Type()), Version: p.Version()})
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
