package plugins

import (
	"context"
	"fmt"
	"log"
)

type Executor struct {
	registry *Registry
}

func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

func (e *Executor) RunPrepare(ctx context.Context, pluginName string, env PluginEnv) error {
	p, err := e.registry.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin lookup: %w", err)
	}
	if err := p.Prepare(ctx, env); err != nil {
		return fmt.Errorf("plugin %s prepare: %w", pluginName, err)
	}
	return nil
}

func (e *Executor) RunExecute(ctx context.Context, pluginName string, env PluginEnv, params map[string]interface{}) (*PluginResult, error) {
	p, err := e.registry.Get(pluginName)
	if err != nil {
		return nil, fmt.Errorf("plugin lookup: %w", err)
	}
	if err := p.Prepare(ctx, env); err != nil {
		return nil, fmt.Errorf("plugin %s prepare: %w", pluginName, err)
	}
	result, err := p.Execute(ctx, env, params)
	if err != nil {
		log.Printf("WARN: plugin %s execute failed, attempting rollback: %v", pluginName, err)
		if rbErr := p.Rollback(ctx, env); rbErr != nil {
			log.Printf("ERROR: plugin %s rollback also failed: %v", pluginName, rbErr)
		}
		return nil, fmt.Errorf("plugin %s execute: %w", pluginName, err)
	}
	return result, nil
}

func (e *Executor) RunTeardown(ctx context.Context, pluginName string, env PluginEnv) error {
	p, err := e.registry.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin lookup: %w", err)
	}
	if err := p.Teardown(ctx, env); err != nil {
		return fmt.Errorf("plugin %s teardown: %w", pluginName, err)
	}
	return nil
}

func (e *Executor) RunJoin(ctx context.Context, pluginName string, env PluginEnv, newNode PluginNode) error {
	p, err := e.registry.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin lookup: %w", err)
	}
	if err := p.Join(ctx, env, newNode); err != nil {
		return fmt.Errorf("plugin %s join: %w", pluginName, err)
	}
	return nil
}

func (e *Executor) RunLeave(ctx context.Context, pluginName string, env PluginEnv, node PluginNode) error {
	p, err := e.registry.Get(pluginName)
	if err != nil {
		return fmt.Errorf("plugin lookup: %w", err)
	}
	if err := p.Leave(ctx, env, node); err != nil {
		return fmt.Errorf("plugin %s leave: %w", pluginName, err)
	}
	return nil
}
