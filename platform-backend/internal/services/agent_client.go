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
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
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

func (c *AgentClient) DeployInstance(ctx context.Context, hostAddr string, agentPort int, instance *models.Instance, taskID string) (*AgentTaskResult, error) {
	conn, err := getInstanceConnection(instance)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}

	payload := DeployTaskPayload{
		TaskID:     taskID,
		InstanceID: instance.ID,
		Config: map[string]interface{}{
			"deploy_mode": "single",
			"host":        conn.Host,
			"port":        conn.Port,
		},
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

func getInstanceConnection(instance *models.Instance) (*models.InstanceConnection, error) {
	if instance.Connection.Host == "" {
		return nil, fmt.Errorf("connection info not loaded")
	}
	return &instance.Connection, nil
}
