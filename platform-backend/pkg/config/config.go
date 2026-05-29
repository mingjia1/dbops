package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	ServerPort    string
	LogLevel      string
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")

	viper.SetDefault("server_port", "8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("database_url", "postgres://postgres:postgres@localhost:5432/mysql_ops?sslmode=disable")
	viper.SetDefault("redis_url", "localhost:6379")
	viper.SetDefault("jwt_secret", "your-secret-key")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	return &Config{
		ServerPort:    viper.GetString("server_port"),
		LogLevel:      viper.GetString("log_level"),
		DatabaseURL:   viper.GetString("database_url"),
		RedisURL:      viper.GetString("redis_url"),
		JWTSecret:     viper.GetString("jwt_secret"),
	}, nil
}