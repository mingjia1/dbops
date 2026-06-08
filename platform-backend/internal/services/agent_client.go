package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type AgentClient struct {
	httpClient *http.Client
	agentToken string
}

func NewAgentClient(agentToken string) *AgentClient {
	return &AgentClient{
		// B8: 之前 300s timeout 撞死长任务 (backup/upgrade/migration/role-switch).
		// 修: 30s 适合短 op; 长 op 走 fire-and-forget + GetTaskProgress 轮询.
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		agentToken: agentToken,
	}
}

type DeployTaskPayload struct {
	TaskID     string                 `json:"task_id"`
	InstanceID string                 `json:"instance_id"`
	Config     map[string]interface{} `json:"config"`
}

type AgentTaskResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	// Data 携带 verify / 复杂任务的额外字段, 例如 source_count / target_count / errors.
	Data map[string]interface{} `json:"data"`
}

type agentResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *AgentTaskResult `json:"data"`
}

func (c *AgentClient) DeployInstance(ctx context.Context, hostAddr string, agentPort int, instance *models.Instance, taskID, mysqlPassword string) (*AgentTaskResult, error) {
	conn, err := getInstanceConnection(instance)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}

	cfg := map[string]interface{}{
		"deploy_mode": "single",
		"host":        conn.Host,
		"port":        conn.Port,
		"mysql_user":  conn.Username,
		"mysql_pass":  mysqlPassword,
	}
	// Forward version-agnostic install fields. They are optional — when absent
	// the agent falls back to whatever is on PATH (legacy behaviour).
	if conn.VersionID != "" {
		cfg["version_id"] = conn.VersionID
	}
	if conn.PackageURL != "" {
		cfg["package_url"] = conn.PackageURL
	}
	if conn.Basedir != "" {
		cfg["basedir"] = conn.Basedir
	}
	if conn.Datadir != "" {
		cfg["datadir"] = conn.Datadir
	}
	if conn.OSUser != "" {
		cfg["os_user"] = conn.OSUser
	}

	payload := DeployTaskPayload{
		TaskID:     taskID,
		InstanceID: instance.ID,
		Config:     cfg,
	}

	return c.callAgent(ctx, hostAddr, agentPort, "/agent/tasks/deploy", payload)
}

func (c *AgentClient) DeployMasterSlave(ctx context.Context, hostAddr string, agentPort int, config map[string]interface{}, taskID string) (*AgentTaskResult, error) {
	payload := DeployTaskPayload{
		TaskID:     taskID,
		InstanceID: "",
		Config:     config,
	}
	payload.Config["deploy_mode"] = "master-slave"

	return c.callAgent(ctx, hostAddr, agentPort, "/agent/tasks/deploy", payload)
}

func (c *AgentClient) ExecuteHealthCheck(ctx context.Context, hostAddr string, agentPort int, instanceID string) (*AgentTaskResult, error) {
	url := fmt.Sprintf("http://%s:%d/agent/tasks/health-check?instance_id=%s", hostAddr, agentPort, instanceID)
	return c.callAgentGet(ctx, url)
}

func (c *AgentClient) ExecuteBackup(ctx context.Context, hostAddr string, agentPort int, config map[string]interface{}, taskID, instanceID string) (*AgentTaskResult, error) {
	payload := DeployTaskPayload{
		TaskID:     taskID,
		InstanceID: instanceID,
		Config:     config,
	}
	return c.callAgent(ctx, hostAddr, agentPort, "/agent/tasks/backup", payload)
}

func (c *AgentClient) callAgent(ctx context.Context, hostAddr string, agentPort int, path string, payload interface{}) (*AgentTaskResult, error) {
	url := fmt.Sprintf("http://%s:%d%s", hostAddr, agentPort, path)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// P0: 把 backend 中间件生成的 trace_id 透传给 agent, 跨组件排障可串联.
	if tid := ctx.Value("trace_id"); tid != nil {
		if s, ok := tid.(string); ok && s != "" {
			req.Header.Set("X-Trace-Id", s)
		}
	}
	// P1-2: 携带 agent_token 鉴权, 与 Agent 端校验.
	if c.agentToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.agentToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result agentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("agent error (code %d): %s", result.Code, result.Message)
	}

	return result.Data, nil
}

func (c *AgentClient) callAgentGet(ctx context.Context, url string) (*AgentTaskResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.agentToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.agentToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result agentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// GetTaskProgress B8: 长任务 (backup/upgrade/migration/role-switch) 现在用
// fire-and-forget 派发 (调 callAgent 拿到 202 + task_id 后立即返), 前端用此方法
// 轮询 task 进度, 不再被 300s httpClient.Timeout 撞死.
func (c *AgentClient) GetTaskProgress(ctx context.Context, host string, port int, taskID string) (*AgentTaskResult, error) {
	url := fmt.Sprintf("http://%s:%d/agent/tasks/%s/progress", host, port, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.agentToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.agentToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent progress request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("agent has no record of task %s (may have completed and been purged)", taskID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var result agentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result.Data, nil
}

func getInstanceConnection(instance *models.Instance) (*models.InstanceConnection, error) {
	if instance.Connection.Host == "" {
		return nil, fmt.Errorf("connection info not loaded")
	}
	return &instance.Connection, nil
}
