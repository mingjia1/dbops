package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	AgentPort     string
	PlatformURL   string
	AgentToken    string
	LogLevel      string
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

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	return &Config{
		AgentPort:     viper.GetString("agent_port"),
		PlatformURL:   viper.GetString("platform_url"),
		AgentToken:    viper.GetString("agent_token"),
		LogLevel:      viper.GetString("log_level"),
	}, nil
}