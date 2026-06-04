package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	ServerPort     string
	LogLevel       string
	DatabaseURL    string
	SQLitePath     string
	RedisURL       string
	RedisPassword  string
	RedisDB        int
	JWTSecret      string
	EncryptionKey  string
	ClickHouseURL  string
	DataDir        string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")

	viper.SetDefault("server_port", "8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("database_url", "root:password@tcp(localhost:3306)/mysql_ops?charset=utf8mb4&parseTime=true&loc=Local")
	viper.SetDefault("sqlite_path", "") // 空表示使用 <DataDir>/dbops.db
	viper.SetDefault("redis_url", "localhost:6379")
	viper.SetDefault("redis_password", "")
	viper.SetDefault("redis_db", 0)
	viper.SetDefault("jwt_secret", "your-secret-key")
	viper.SetDefault("encryption_key", "mysql-ops-platform-32byte-aes-key!!")
	viper.SetDefault("clickhouse_url", "clickhouse://default@localhost:9000/default")
	viper.SetDefault("data_dir", "./data")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	return &Config{
		ServerPort:    viper.GetString("server_port"),
		LogLevel:      viper.GetString("log_level"),
		DatabaseURL:   viper.GetString("database_url"),
		SQLitePath:    viper.GetString("sqlite_path"),
		RedisURL:      viper.GetString("redis_url"),
		RedisPassword: viper.GetString("redis_password"),
		RedisDB:       viper.GetInt("redis_db"),
		JWTSecret:     viper.GetString("jwt_secret"),
		EncryptionKey: viper.GetString("encryption_key"),
		ClickHouseURL: viper.GetString("clickhouse_url"),
		DataDir:       viper.GetString("data_dir"),
	}, nil
}