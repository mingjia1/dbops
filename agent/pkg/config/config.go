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
	Relay       RelayConfig
}

type RelayConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	CacheDir      string `mapstructure:"cache_dir"`
	RelayHost     string `mapstructure:"relay_host"`
	RelayPort     int    `mapstructure:"relay_port"`
	RelayToken    string `mapstructure:"relay_token"`
	MaxCacheSizeGB int   `mapstructure:"max_cache_size_gb"`
	CacheExpireHours int `mapstructure:"cache_expire_hours"`
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
	viper.SetDefault("relay.enabled", false)
	viper.SetDefault("relay.cache_dir", "/data/relay/packages")
	viper.SetDefault("relay.relay_port", 9091)
	viper.SetDefault("relay.max_cache_size_gb", 50)
	viper.SetDefault("relay.cache_expire_hours", 168)

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
		Relay: RelayConfig{
			Enabled:          viper.GetBool("relay.enabled"),
			CacheDir:         viper.GetString("relay.cache_dir"),
			RelayHost:        viper.GetString("relay.relay_host"),
			RelayPort:        viper.GetInt("relay.relay_port"),
			RelayToken:       viper.GetString("relay.relay_token"),
			MaxCacheSizeGB:   viper.GetInt("relay.max_cache_size_gb"),
			CacheExpireHours: viper.GetInt("relay.cache_expire_hours"),
		},
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