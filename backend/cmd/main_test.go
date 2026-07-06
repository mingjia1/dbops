package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/pkg/config"
)

func TestValidateSecretsRejectsWeakValuesByDefault(t *testing.T) {
	t.Setenv("DBOPS_ALLOW_INSECURE_DEV_SECRETS", "")

	err := validateSecrets(&config.Config{
		DatabaseURL:   "app:strong-random-value@tcp(db:3306)/dbops?parseTime=true",
		JWTSecret:     "test_jwt_secret_key_for_ha_cluster_testing_2024",
		EncryptionKey: strings.Repeat("a", 32),
		AgentToken:    strings.Repeat("b", 16),
	})

	if err == nil {
		t.Fatal("expected weak test JWT secret to be rejected")
	}
	if !strings.Contains(err.Error(), "insecure test/default value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSecretsAllowsWeakValuesWithExplicitDevOverride(t *testing.T) {
	t.Setenv("DBOPS_ALLOW_INSECURE_DEV_SECRETS", "1")

	err := validateSecrets(&config.Config{
		DatabaseURL:   "root:123456@tcp(10.1.81.42:23306)/dbops_platform?parseTime=true",
		JWTSecret:     "test_jwt_secret_key_for_ha_cluster_testing_2024",
		EncryptionKey: "test_encryption_key_for_ha_cluster_testing_2024",
		AgentToken:    "test_agent_token_for_ha_cluster_2024",
	})

	if err != nil {
		t.Fatalf("expected explicit dev override to allow weak local values, got %v", err)
	}
}

func TestValidateSecretsRejectsPlaceholdersEvenWithDevOverride(t *testing.T) {
	t.Setenv("DBOPS_ALLOW_INSECURE_DEV_SECRETS", "1")

	err := validateSecrets(&config.Config{
		DatabaseURL:   "app:strong-random-value@tcp(db:3306)/dbops?parseTime=true",
		JWTSecret:     "PLEASE-CHANGE-JWT-SECRET-12345678901234567890",
		EncryptionKey: strings.Repeat("a", 32),
		AgentToken:    strings.Repeat("b", 16),
	})

	if err == nil {
		t.Fatal("expected placeholder secret to be rejected")
	}
}

func TestWriteBootstrapAdminCredentialWritesFileWithoutLoggingPassword(t *testing.T) {
	dir := t.TempDir()

	path, err := writeBootstrapAdminCredential(dir, "admin", "Bootstrap#1234")
	if err != nil {
		t.Fatalf("writeBootstrapAdminCredential returned error: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("credential path %q not under temp dir %q", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read credential file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "username=admin") || !strings.Contains(content, "password=Bootstrap#1234") {
		t.Fatalf("unexpected credential file content: %q", content)
	}
}

func TestRequireAgentToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/internal/metrics/ingest", requireAgentToken("agent-token-123456"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/metrics/ingest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/metrics/ingest", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/metrics/ingest", nil)
	req.Header.Set("Authorization", "Bearer agent-token-123456")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200", w.Code)
	}
}
