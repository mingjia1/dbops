package config

import (
	"os"
	"strings"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"http://localhost:3000,http://127.0.0.1:3000", []string{"http://localhost:3000", "http://127.0.0.1:3000"}},
	}

	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q) = %v (len=%d), want %v (len=%d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Clean env vars that Load() reads but we want default values for.
	for _, key := range backendEnvKeysForTest() {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Server port default
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "8080")
	}
	// Log level default
	if cfg.LogLevel != "warning" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warning")
	}
	// Storage mode default - may be overridden by local config.yaml
	// Accept both "auto" (true default) and whatever the project config sets
	if cfg.StorageMode == "" {
		t.Error("StorageMode should not be empty")
	}
	// AllowedOrigins should have at least 1 entry
	if len(cfg.AllowedOrigins) == 0 {
		t.Errorf("AllowedOrigins is empty, want at least 1")
	}
	// Cluster defaults
	if cfg.ClusterDefaults.ReplicationUser == "" {
		t.Errorf("ClusterDefaults.ReplicationUser is empty")
	}
	if cfg.ClusterDefaults.SSHUser == "" {
		t.Errorf("ClusterDefaults.SSHUser is empty")
	}
	// AI defaults
	if cfg.AIModel == "" {
		t.Errorf("AIModel is empty")
	}
	if cfg.AIMaxTokens <= 0 {
		t.Errorf("AIMaxTokens = %d, want > 0", cfg.AIMaxTokens)
	}
	// Server timeouts
	if cfg.ServerTimeouts.ReadTimeoutSec <= 0 {
		t.Errorf("ReadTimeoutSec = %d, want > 0", cfg.ServerTimeouts.ReadTimeoutSec)
	}
	// DataDir defaults to "../db"
	if cfg.DataDir == "" {
		t.Error("DataDir is empty, want a default path")
	}
	// Redis defaults (may be empty if config file doesn't set it)
	// Not asserting a specific value because local config.yaml may override
}

func TestLoad_EnvOverrides(t *testing.T) {
	// Set env vars that should override both defaults and config.yaml
	t.Setenv("DBOPS_DB_URL", "mysql://test:test@localhost/testdb")
	t.Setenv("DBOPS_JWT_SECRET", "test-jwt-secret-from-env")
	t.Setenv("DBOPS_ENCRYPTION_KEY", "test-enc-key-32bytes!")
	t.Setenv("DBOPS_AGENT_TOKEN", "test-agent-token-from-env")
	t.Setenv("DBOPS_STORAGE_MODE", "sqlite")
	t.Setenv("DBOPS_SERVER_PORT", "18080")
	t.Setenv("DBOPS_TLS_CERT_PATH", "/secure/tls.crt")
	t.Setenv("DBOPS_TLS_KEY_PATH", "/secure/tls.key")
	t.Setenv("DBOPS_LOG_LEVEL", "debug")
	t.Setenv("DBOPS_SQLITE_PATH", "/data/dbops.db")
	t.Setenv("DBOPS_REDIS_URL", "redis.local:6379")
	t.Setenv("DBOPS_REDIS_PASSWORD", "redis-secret")
	t.Setenv("DBOPS_REDIS_DB", "2")
	t.Setenv("DBOPS_CLICKHOUSE_URL", "clickhouse://user:pass@clickhouse:9000/dbops")
	t.Setenv("DBOPS_DATA_DIR", "/data/dbops")
	t.Setenv("DBOPS_ALLOWED_ORIGINS", "http://localhost:5173,https://ops.example.com")
	t.Setenv("DBOPS_AI_API_KEY", "sk-test-key")
	t.Setenv("DBOPS_AI_BASE_URL", "https://test.ai.com")
	t.Setenv("DBOPS_AI_MODEL", "gpt-test")
	t.Setenv("DBOPS_AI_MAX_TOKENS", "4096")
	t.Setenv("DBOPS_REPLICATION_USER", "repl_env")
	t.Setenv("DBOPS_REPLICATION_PASS", "repl-secret")
	t.Setenv("DBOPS_SST_USER", "sst_env")
	t.Setenv("DBOPS_SST_PASS", "sst-secret")
	t.Setenv("DBOPS_SSH_USER", "mysqlops")
	t.Setenv("DBOPS_SERVER_READ_TIMEOUT_SEC", "11")
	t.Setenv("DBOPS_SERVER_WRITE_TIMEOUT_SEC", "22")
	t.Setenv("DBOPS_SERVER_IDLE_TIMEOUT_SEC", "33")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DatabaseURL != "mysql://test:test@localhost/testdb" {
		t.Errorf("DatabaseURL = %q, want env override", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "test-jwt-secret-from-env" {
		t.Errorf("JWTSecret = %q, want env override", cfg.JWTSecret)
	}
	if cfg.EncryptionKey != "test-enc-key-32bytes!" {
		t.Errorf("EncryptionKey = %q, want env override", cfg.EncryptionKey)
	}
	if cfg.AgentToken != "test-agent-token-from-env" {
		t.Errorf("AgentToken = %q, want env override", cfg.AgentToken)
	}
	if cfg.StorageMode != "sqlite" {
		t.Errorf("StorageMode = %q, want sqlite (env override)", cfg.StorageMode)
	}
	if cfg.ServerPort != "18080" {
		t.Errorf("ServerPort = %q, want env override", cfg.ServerPort)
	}
	if cfg.TLSCertPath != "/secure/tls.crt" || cfg.TLSKeyPath != "/secure/tls.key" {
		t.Errorf("TLS paths = %q/%q, want env override", cfg.TLSCertPath, cfg.TLSKeyPath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want env override", cfg.LogLevel)
	}
	if cfg.SQLitePath != "/data/dbops.db" {
		t.Errorf("SQLitePath = %q, want env override", cfg.SQLitePath)
	}
	if cfg.RedisURL != "redis.local:6379" || cfg.RedisPassword != "redis-secret" || cfg.RedisDB != 2 {
		t.Errorf("Redis config = %q/%q/%d, want env override", cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB)
	}
	if cfg.ClickHouseURL != "clickhouse://user:pass@clickhouse:9000/dbops" {
		t.Errorf("ClickHouseURL = %q, want env override", cfg.ClickHouseURL)
	}
	if cfg.DataDir != "/data/dbops" {
		t.Errorf("DataDir = %q, want env override", cfg.DataDir)
	}
	if got := strings.Join(cfg.AllowedOrigins, ","); got != "http://localhost:5173,https://ops.example.com" {
		t.Errorf("AllowedOrigins = %q, want env override", got)
	}
	if cfg.AIAPIKey != "sk-test-key" {
		t.Errorf("AIAPIKey = %q, want env override", cfg.AIAPIKey)
	}
	if cfg.AIBaseURL != "https://test.ai.com" {
		t.Errorf("AIBaseURL = %q, want env override", cfg.AIBaseURL)
	}
	if cfg.AIModel != "gpt-test" || cfg.AIMaxTokens != 4096 {
		t.Errorf("AI model config = %q/%d, want env override", cfg.AIModel, cfg.AIMaxTokens)
	}
	if cfg.ClusterDefaults.ReplicationUser != "repl_env" || cfg.ClusterDefaults.ReplicationPass != "repl-secret" ||
		cfg.ClusterDefaults.SSTUser != "sst_env" || cfg.ClusterDefaults.SSTPass != "sst-secret" || cfg.ClusterDefaults.SSHUser != "mysqlops" {
		t.Errorf("Cluster defaults = %#v, want env override", cfg.ClusterDefaults)
	}
	if cfg.ServerTimeouts.ReadTimeoutSec != 11 || cfg.ServerTimeouts.WriteTimeoutSec != 22 || cfg.ServerTimeouts.IdleTimeoutSec != 33 {
		t.Errorf("ServerTimeouts = %#v, want env override", cfg.ServerTimeouts)
	}
}

func TestLoad_AllowedOriginsParsing(t *testing.T) {
	// Verify that AllowedOrigins is always a non-empty slice of valid-looking URLs
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.AllowedOrigins) == 0 {
		t.Error("AllowedOrigins should have at least 1 entry")
	}

	for _, origin := range cfg.AllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			t.Error("AllowedOrigins contains empty entry")
			continue
		}
		if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			t.Errorf("AllowedOrigins entry %q should start with http:// or https://", origin)
		}
	}
}

func backendEnvKeysForTest() []string {
	return []string{
		"DBOPS_SERVER_PORT",
		"DBOPS_TLS_CERT_PATH",
		"DBOPS_TLS_KEY_PATH",
		"DBOPS_LOG_LEVEL",
		"DBOPS_DB_URL",
		"DBOPS_SQLITE_PATH",
		"DBOPS_STORAGE_MODE",
		"DBOPS_REDIS_URL",
		"DBOPS_REDIS_PASSWORD",
		"DBOPS_REDIS_DB",
		"DBOPS_JWT_SECRET",
		"DBOPS_ENCRYPTION_KEY",
		"DBOPS_AGENT_TOKEN",
		"DBOPS_CLICKHOUSE_URL",
		"DBOPS_DATA_DIR",
		"DBOPS_ALLOWED_ORIGINS",
		"DBOPS_AI_BASE_URL",
		"DBOPS_AI_API_KEY",
		"DBOPS_AI_MODEL",
		"DBOPS_AI_MAX_TOKENS",
		"DBOPS_REPLICATION_USER",
		"DBOPS_REPLICATION_PASS",
		"DBOPS_SST_USER",
		"DBOPS_SST_PASS",
		"DBOPS_SSH_USER",
		"DBOPS_SERVER_READ_TIMEOUT_SEC",
		"DBOPS_SERVER_WRITE_TIMEOUT_SEC",
		"DBOPS_SERVER_IDLE_TIMEOUT_SEC",
	}
}
