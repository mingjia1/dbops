package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-agent/internal/executor"
)

type fakeMiddlewareSetupExecutor struct {
	keepalivedReq   executor.DeployTaskRequest
	proxysqlReq     executor.DeployTaskRequest
	keepalivedReply *executor.TaskResult
	proxysqlReply   *executor.TaskResult
}

func (f *fakeMiddlewareSetupExecutor) ExecuteKeepalivedSetup(_ context.Context, req executor.DeployTaskRequest) (*executor.TaskResult, error) {
	f.keepalivedReq = req
	if f.keepalivedReply != nil {
		return f.keepalivedReply, nil
	}
	return &executor.TaskResult{TaskID: req.TaskID, Status: "completed", Progress: 100, Message: "ok", Timestamp: time.Now()}, nil
}

func (f *fakeMiddlewareSetupExecutor) ExecuteProxySQLSetup(_ context.Context, req executor.DeployTaskRequest) (*executor.TaskResult, error) {
	f.proxysqlReq = req
	if f.proxysqlReply != nil {
		return f.proxysqlReply, nil
	}
	return &executor.TaskResult{TaskID: req.TaskID, Status: "completed", Progress: 100, Message: "ok", Timestamp: time.Now()}, nil
}

func TestMiddlewareSetupRoutesBindKeepalivedRequest(t *testing.T) {
	router, fakeExec := newMiddlewareSetupRouteTestRouter()
	body := []byte(`{"task_id":"keepalived-task","config":{"vip":"10.1.1.100","vip_interface":"eth0","priority":100,"role":"MASTER","mysql_port":3306}}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/agent/tasks/keepalived-setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fakeExec.keepalivedReq.TaskID != "keepalived-task" {
		t.Fatalf("unexpected task id: %s", fakeExec.keepalivedReq.TaskID)
	}
	if fakeExec.keepalivedReq.Config["vip"] != "10.1.1.100" {
		t.Fatalf("vip was not bound into config: %#v", fakeExec.keepalivedReq.Config)
	}
	if got := int(fakeExec.keepalivedReq.Config["mysql_port"].(float64)); got != 3306 {
		t.Fatalf("unexpected mysql_port: %d", got)
	}
}

func TestMiddlewareSetupRoutesBindProxySQLRequest(t *testing.T) {
	router, fakeExec := newMiddlewareSetupRouteTestRouter()
	body := []byte(`{"task_id":"proxysql-task","config":{"proxy_host":"10.1.1.20","proxy_port":6033,"backends":[{"host":"10.1.1.11","port":3306}],"admin_user":"root","admin_pass":"Root#1234"}}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/agent/tasks/proxysql-setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fakeExec.proxysqlReq.TaskID != "proxysql-task" {
		t.Fatalf("unexpected task id: %s", fakeExec.proxysqlReq.TaskID)
	}
	if fakeExec.proxysqlReq.Config["proxy_host"] != "10.1.1.20" {
		t.Fatalf("proxy_host was not bound into config: %#v", fakeExec.proxysqlReq.Config)
	}
	backends, ok := fakeExec.proxysqlReq.Config["backends"].([]interface{})
	if !ok || len(backends) != 1 {
		t.Fatalf("backends were not bound as a list: %#v", fakeExec.proxysqlReq.Config["backends"])
	}
}

func TestMiddlewareSetupRoutesReturn500ForFailedTaskResult(t *testing.T) {
	router, fakeExec := newMiddlewareSetupRouteTestRouter()
	fakeExec.proxysqlReply = &executor.TaskResult{TaskID: "proxysql-task", Status: "failed", Progress: 20, Message: "install failed", Timestamp: time.Now()}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/agent/tasks/proxysql-setup", bytes.NewReader([]byte(`{"task_id":"proxysql-task","config":{"proxy_host":"10.1.1.20"}}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCompatAccountRoutesRequireBearerToken(t *testing.T) {
	router := newCompatAccountRouteTestRouter()

	for _, path := range []string{"/api/v1/accounts/setup", "/api/v1/accounts/rotate"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s expected 401 without token, got %d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestCompatAccountRoutesRejectInvalidBearerToken(t *testing.T) {
	router := newCompatAccountRouteTestRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/setup", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCompatAccountSetupAllowsAuthorizedEmptySetup(t *testing.T) {
	router := newCompatAccountRouteTestRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/setup", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-agent-token")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized no-op setup, got %d body=%s", w.Code, w.Body.String())
	}
}

func newMiddlewareSetupRouteTestRouter() (*gin.Engine, *fakeMiddlewareSetupExecutor) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	fakeExec := &fakeMiddlewareSetupExecutor{}
	registerMiddlewareSetupTaskRoutes(router.Group("/agent/tasks"), fakeExec)
	return router, fakeExec
}

func newCompatAccountRouteTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerCompatAccountRoutes(router.Group("/api/v1/accounts", agentAuthMiddleware("test-agent-token")), executor.NewAccountManager())
	return router
}
