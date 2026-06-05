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
	StorageMode    string
	RedisURL       string
	RedisPassword  string
	RedisDB        int
	JWTSecret      string
	EncryptionKey  string
	AgentToken     string
	ClickHouseURL  string
	DataDir        string
	AllowedOrigins []string
	ClusterDefaults ClusterDefaults
}

type ClusterDefaults struct {
	ReplicationUser string
	ReplicationPass string
	SSTUser         string
	SSTPass         string
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
	viper.SetDefault("clickhouse_url", "clickhouse://default@localhost:9000/default")
	viper.SetDefault("data_dir", "./data")
	// 存储模式: auto = 先试 MySQL 再回退 SQLite; mysql = 强制 MySQL (连不上 fail);
	// sqlite = 强制 SQLite. 默认 auto, 显式声明避免静默降级.
	viper.SetDefault("storage_mode", "auto")
	// CORS 白名单 (逗号分隔), 默认仅本机前端.
	viper.SetDefault("allowed_origins", "http://localhost:3000,http://127.0.0.1:3000")
	// 集群部署默认凭据 (生产应通过 secret 管理覆盖).
	viper.SetDefault("cluster_defaults.replication_user", "repl")
	viper.SetDefault("cluster_defaults.replication_pass", "Repl#2024!ChangeMe")
	viper.SetDefault("cluster_defaults.sst_user", "sstuser")
	viper.SetDefault("cluster_defaults.sst_pass", "Sst#2024!ChangeMe")
	// Agent 鉴权 token (Agent <-> Backend 通信). 强制要求设置, 无默认值.
	viper.SetDefault("agent_token", "")
	// JWT / 加密密钥故意不设默认值, 必须从 config.yaml 显式注入, 启动时校验.

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
		StorageMode:   viper.GetString("storage_mode"),
		RedisURL:      viper.GetString("redis_url"),
		RedisPassword: viper.GetString("redis_password"),
		RedisDB:       viper.GetInt("redis_db"),
		JWTSecret:     viper.GetString("jwt_secret"),
		EncryptionKey: viper.GetString("encryption_key"),
		AgentToken:    viper.GetString("agent_token"),
		ClickHouseURL: viper.GetString("clickhouse_url"),
		DataDir:       viper.GetString("data_dir"),
		AllowedOrigins: splitCSV(viper.GetString("allowed_origins")),
		ClusterDefaults: ClusterDefaults{
			ReplicationUser: viper.GetString("cluster_defaults.replication_user"),
			ReplicationPass: viper.GetString("cluster_defaults.replication_pass"),
			SSTUser:         viper.GetString("cluster_defaults.sst_user"),
			SSTPass:         viper.GetString("cluster_defaults.sst_pass"),
		},
	}, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if start < i {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}