package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type EnvironmentCheckService struct {
	hostRepo    *repositories.HostRepository
	agentClient *AgentClient
	encKey      string
}

func NewEnvironmentCheckService(hostRepo *repositories.HostRepository, agentClient *AgentClient, encKey string) *EnvironmentCheckService {
	return &EnvironmentCheckService{
		hostRepo:    hostRepo,
		agentClient: agentClient,
		encKey:      encKey,
	}
}

type EnvironmentCheckRequest struct {
	Hosts   []HostConfig `json:"hosts"`
	HostIDs []string     `json:"host_ids"`
}

type HostConfig struct {
	Host      string `json:"host" binding:"required"`
	Port      int    `json:"port" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	AgentPort int    `json:"agent_port,omitempty"`
}

type EnvironmentCheckResult struct {
	CheckID   string        `json:"check_id"`
	Status    string        `json:"status"`
	Results   []CheckResult `json:"results"`
	CreatedAt time.Time     `json:"created_at"`
}

type CheckResult struct {
	Category   string `json:"category"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Passed     bool   `json:"passed"`
	Value      string `json:"value"`
	Suggestion string `json:"suggestion"`
}

func (s *EnvironmentCheckService) Execute(ctx context.Context, req EnvironmentCheckRequest) (*EnvironmentCheckResult, error) {
	hosts, err := s.resolveHosts(ctx, req)
	if err != nil {
		return nil, err
	}

	result := &EnvironmentCheckResult{
		CheckID:   fmt.Sprintf("check-%d", time.Now().Unix()),
		Status:    "running",
		CreatedAt: time.Now(),
		Results:   []CheckResult{},
	}

	for _, host := range hosts {
		agentPort := host.AgentPort
		if agentPort == 0 {
			agentPort = 9090
		}
		agentResult := s.checkHost(host)
		result.Results = append(result.Results, agentResult...)

		healthResult, err := s.agentClient.ExecuteHealthCheck(ctx, host.Host, agentPort, "")
		if err != nil {
			result.Results = append(result.Results, CheckResult{
				Category:   "agent",
				Name:       "agent_connectivity",
				Status:     "failed",
				Passed:     false,
				Value:      fmt.Sprintf("%s:%d", host.Host, agentPort),
				Suggestion: "请确认 Agent 已在目标主机上启动（端口 9090）",
			})
			continue
		}

		result.Results = append(result.Results, CheckResult{
			Category:   "agent",
			Name:       "agent_connectivity",
			Status:     "passed",
			Passed:     true,
			Value:      fmt.Sprintf("%s:%d - %s", host.Host, agentPort, healthResult.Status),
			Suggestion: "",
		})
		result.Results = removeAgentCollectedPlaceholders(result.Results)
		result.Results = append(result.Results, agentSystemResults(healthResult.Data)...)
	}

	result.Status = "completed"
	return result, nil
}

func (s *EnvironmentCheckService) resolveHosts(ctx context.Context, req EnvironmentCheckRequest) ([]HostConfig, error) {
	if len(req.Hosts) > 0 {
		return req.Hosts, nil
	}
	if len(req.HostIDs) == 0 {
		return nil, errors.New("hosts or host_ids is required")
	}
	if s.hostRepo == nil {
		return nil, errors.New("host repository is not configured")
	}

	hosts := make([]HostConfig, 0, len(req.HostIDs))
	for _, id := range req.HostIDs {
		host, err := s.hostRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get host %s: %w", id, err)
		}
		password, err := utils.Decrypt(host.SSHCredential, s.encKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt SSH credential for host %s: %w", host.Name, err)
		}
		if password == "" {
			return nil, fmt.Errorf("host %s has no SSH credential; please edit the host and save SSH credential", host.Name)
		}
		hosts = append(hosts, HostConfig{
			Host:      host.Address,
			Port:      host.SSHPort,
			Username:  host.SSHUser,
			Password:  password,
			AgentPort: host.AgentPort,
		})
	}
	return hosts, nil
}

// checkHost performs local reachability checks and leaves host metrics as unknown
// when they require agent-side collection.
func (s *EnvironmentCheckService) checkHost(host HostConfig) []CheckResult {
	results := []CheckResult{}

	addr := net.JoinHostPort(host.Host, fmt.Sprintf("%d", host.Port))
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		results = append(results, CheckResult{
			Category:   "network",
			Name:       "port_reachable",
			Status:     "failed",
			Passed:     false,
			Value:      fmt.Sprintf("%s: %v", addr, err),
			Suggestion: "确认 MySQL 已启动，且目标端口对 backend 可达",
		})
	} else {
		_ = conn.Close()
		results = append(results, CheckResult{
			Category: "network",
			Name:     "port_reachable",
			Status:   "passed",
			Passed:   true,
			Value:    fmt.Sprintf("%s: ok", addr),
		})
	}

	results = append(results,
		CheckResult{Category: "hardware", Name: "cpu_cores", Status: "unknown", Passed: false, Value: "agent_collected"},
		CheckResult{Category: "hardware", Name: "memory_size", Status: "unknown", Passed: false, Value: "agent_collected"},
		CheckResult{Category: "hardware", Name: "disk_space", Status: "unknown", Passed: false, Value: "agent_collected"},
		CheckResult{Category: "os", Name: "kernel_version", Status: "unknown", Passed: false, Value: "agent_collected"},
		CheckResult{Category: "dependency", Name: "libaio", Status: "unknown", Passed: false, Value: "agent_collected"},
	)
	return results

	for _, item := range []struct{ cat, name, suggestion string }{} {
		_ = item
	}
	for _, item := range []struct{ cat, name, suggestion string }{
		{"hardware", "cpu_cores", "需要 Agent 端采集"},
		{"hardware", "memory_size", "需要 Agent 端采集"},
		{"hardware", "disk_space", "需要 Agent 端采集"},
		{"os", "kernel_version", "需要 Agent 端采集"},
		{"dependency", "libaio", "需要 Agent 端采集"},
	} {
		results = append(results, CheckResult{
			Category:   item.cat,
			Name:       item.name,
			Status:     "unknown",
			Passed:     false,
			Value:      "not_collected",
			Suggestion: item.suggestion,
		})
	}

	_ = runtime.GOOS
	_ = runtime.NumCPU()

	return results
}

func (s *EnvironmentCheckService) GetByID(ctx context.Context, checkID string) (*EnvironmentCheckResult, error) {
	return &EnvironmentCheckResult{
		CheckID:   checkID,
		Status:    "completed",
		CreatedAt: time.Now(),
		Results:   []CheckResult{},
	}, nil
}

func (s *EnvironmentCheckService) Export(ctx context.Context, checkID string) (string, error) {
	report := fmt.Sprintf("# Environment Check Report: %s\n\nStatus: completed\nDate: %s\n",
		checkID, time.Now().Format("2006-01-02 15:04:05"))
	return report, nil
}

func agentSystemResults(data map[string]interface{}) []CheckResult {
	items := []struct {
		category   string
		name       string
		key        string
		suggestion string
	}{
		{"os", "os_release", "os_release", "Agent 未采集到操作系统发行版"},
		{"os", "kernel_version", "kernel_version", "Agent 未采集到内核版本"},
		{"hardware", "cpu_cores", "cpu_cores", "Agent 未采集到 CPU 核数"},
		{"hardware", "memory_size", "memory_size", "Agent 未采集到内存信息"},
		{"hardware", "disk_space", "disk_space", "Agent 未采集到磁盘信息"},
		{"dependency", "libaio", "libaio", "请安装 libaio 依赖"},
	}
	out := make([]CheckResult, 0, len(items))
	for _, item := range items {
		value := stringMapValue(data, item.key)
		passed := value != "" && value != "not_found"
		status := "passed"
		suggestion := ""
		if !passed {
			status = "failed"
			if value == "" {
				value = "not_collected"
			}
			suggestion = item.suggestion
		}
		out = append(out, CheckResult{
			Category:   item.category,
			Name:       item.name,
			Status:     status,
			Passed:     passed,
			Value:      value,
			Suggestion: suggestion,
		})
	}
	return out
}

func stringMapValue(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func removeAgentCollectedPlaceholders(results []CheckResult) []CheckResult {
	out := results[:0]
	for _, result := range results {
		if result.Value == "agent_collected" {
			continue
		}
		out = append(out, result)
	}
	return out
}
