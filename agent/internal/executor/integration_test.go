package executor_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-agent/internal/executor"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	toolInstaller := executor.NewToolInstaller()
	environmentChecker := executor.NewEnvironmentChecker()
	relayManager := executor.NewRelayPackageManager("/tmp/test-relay-cache", 1, 24)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.POST("/agent/tasks/blank-host-init", func(c *gin.Context) {
		var req executor.BlankHostInitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
			return
		}
		result := toolInstaller.InitBlankHost(c.Request.Context(), req)
		code := 200
		if result.Status == "failed" {
			code = 500
		}
		c.JSON(code, gin.H{"code": code, "message": result.Message, "data": result})
	})

	r.POST("/agent/tasks/general-cluster-init", func(c *gin.Context) {
		var req executor.GeneralClusterInitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
			return
		}
		result := toolInstaller.InitGeneralCluster(c.Request.Context(), req)
		code := 200
		if result.Status == "failed" {
			code = 500
		}
		c.JSON(code, gin.H{"code": code, "message": result.Message, "data": result})
	})

	r.POST("/agent/tasks/check-environment", func(c *gin.Context) {
		var req executor.EnvironmentCheckRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": 400, "message": "Invalid request"})
			return
		}
		result := environmentChecker.CheckEnvironment(c.Request.Context(), req)
		code := 200
		if result.Status == "failed" {
			code = 500
		}
		c.JSON(code, gin.H{"code": code, "message": result.Message, "data": result})
	})

	r.GET("/agent/tasks/check-tools", func(c *gin.Context) {
		tools := []string{"mysql", "mysqld", "xtrabackup"}
		result := toolInstaller.CheckToolsAvailable(tools)
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": result})
	})

	r.GET("/agent/relay/status", func(c *gin.Context) {
		status := relayManager.GetStatus()
		c.JSON(200, gin.H{"code": 200, "message": "success", "data": status})
	})

	return r
}

func TestHealthEndpoint(t *testing.T) {
	r := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", resp["status"])
	}
}

func TestBlankHostInitEndpoint(t *testing.T) {
	r := setupTestRouter()

	body := map[string]interface{}{
		"mysql_version": "8.0",
		"mysql_port":    3306,
		"data_dir":      "/tmp/test-mysql-data",
		"root_password": "TestPass123!",
		"force":         false,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agent/tasks/blank-host-init", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	t.Logf("BlankHostInit response: code=%d body=%s", w.Code, w.Body.String())

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data in response, got: %s", w.Body.String())
	}
	status, _ := data["status"].(string)
	if status == "" {
		t.Fatalf("expected status in data, got: %v", data)
	}
	t.Logf("BlankHostInit status: %s, message: %v", status, data["message"])
}

func TestGeneralClusterInitEndpoint(t *testing.T) {
	r := setupTestRouter()

	nodes := []map[string]interface{}{
		{"host": "192.168.1.10", "port": 3306, "role": "master"},
		{"host": "192.168.1.11", "port": 3306, "role": "slave"},
	}
	body := map[string]interface{}{
		"cluster_type":  "ha",
		"mysql_version": "8.0",
		"nodes":         nodes,
		"root_password": "TestPass123!",
		"repl_user":     "repl",
		"repl_pass":     "ReplPass123!",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agent/tasks/general-cluster-init", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	t.Logf("GeneralClusterInit response: code=%d body=%s", w.Code, w.Body.String())

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data in response, got: %s", w.Body.String())
	}
	status, _ := data["status"].(string)
	if status == "" {
		t.Fatalf("expected status in data, got: %v", data)
	}
	t.Logf("GeneralClusterInit status: %s, message: %v", status, data["message"])
}

func TestCheckEnvironmentEndpoint(t *testing.T) {
	r := setupTestRouter()

	body := map[string]interface{}{
		"check_tools":     true,
		"check_resources": true,
		"check_network":   true,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/agent/tasks/check-environment", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	t.Logf("CheckEnvironment response: code=%d body=%s", w.Code, w.Body.String())

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data in response, got: %s", w.Body.String())
	}
	status, _ := data["status"].(string)
	t.Logf("CheckEnvironment status: %s, os: %v, tools: %v", status, data["os_info"], data["tools"])
}

func TestCheckToolsEndpoint(t *testing.T) {
	r := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/tasks/check-tools", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	t.Logf("CheckTools response: %s", w.Body.String())
}

func TestRelayStatusEndpoint(t *testing.T) {
	r := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agent/relay/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	t.Logf("RelayStatus response: %s", w.Body.String())
}

func TestFullFlowSimulation(t *testing.T) {
	t.Log("=== Full Flow Simulation ===")

	t.Run("Step1-Health", TestHealthEndpoint)
	t.Run("Step2-CheckEnvironment", TestCheckEnvironmentEndpoint)
	t.Run("Step3-CheckTools", TestCheckToolsEndpoint)
	t.Run("Step4-BlankHostInit", TestBlankHostInitEndpoint)
	t.Run("Step5-GeneralClusterInit", TestGeneralClusterInitEndpoint)
	t.Run("Step6-RelayStatus", TestRelayStatusEndpoint)

	t.Log("=== Full Flow Simulation Complete ===")
}
