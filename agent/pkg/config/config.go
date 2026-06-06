package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AgentPort   string
	PlatformURL string
	AgentToken  string
	LogLevel    string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")

	viper.SetDefault("agent_port", "9090")
	viper.SetDefault("platform_url", "http://localhost:8080")
	viper.SetDefault("agent_token", "")
	viper.SetDefault("log_level", "info")

	// A2: 支持通过 env 注入 agent_token, 优先级高于 yaml.
	viper.BindEnv("agent_token", "DBOPS_AGENT_TOKEN")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	cfg := &Config{
		AgentPort:   viper.GetString("agent_port"),
		PlatformURL: viper.GetString("platform_url"),
		AgentToken:  viper.GetString("agent_token"),
		LogLevel:    viper.GetString("log_level"),
	}

	// A2: 启动 fail-fast - 任何缺省 / 占位 / 过短的 agent_token 都拒启动,
	// 避免 agent 起来后 backend 因为 "invalid agent token" 报 500 才发现配置漏.
	if cfg.AgentToken == "" {
		return nil, fmt.Errorf("agent_token must be set; export DBOPS_AGENT_TOKEN or set in config.yaml")
	}
	if len(cfg.AgentToken) < 16 {
		return nil, fmt.Errorf("agent_token must be >= 16 chars (current len=%d); generate with: openssl rand -hex 16", len(cfg.AgentToken))
	}
	if strings.Contains(cfg.AgentToken, "PLEASE-CHANGE") || strings.Contains(cfg.AgentToken, "INJECT_VIA_") {
		return nil, fmt.Errorf("agent_token is a placeholder (%q); set DBOPS_AGENT_TOKEN to a strong random value", cfg.AgentToken)
	}
	return cfg, nil
}