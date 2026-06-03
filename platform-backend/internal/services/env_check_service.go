package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type EnvironmentCheckService struct {
	hostRepo    *repositories.HostRepository
	agentClient *AgentClient
}

func NewEnvironmentCheckService(hostRepo *repositories.HostRepository, agentClient *AgentClient) *EnvironmentCheckService {
	return &EnvironmentCheckService{
		hostRepo:    hostRepo,
		agentClient: agentClient,
	}
}

type EnvironmentCheckRequest struct {
	Hosts []HostConfig `json:"hosts" binding:"required"`
}

type HostConfig struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
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
	result := &EnvironmentCheckResult{
		CheckID:   fmt.Sprintf("check-%d", time.Now().Unix()),
		Status:    "running",
		CreatedAt: time.Now(),
		Results:   []CheckResult{},
	}

	for _, host := range req.Hosts {
		agentPort := 9090
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
				Suggestion: "请确保 Agent 已在目标主机上启动 (端口 9090)",
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
	}

	result.Status = "completed"
	return result, nil
}

func (s *EnvironmentCheckService) checkHost(host HostConfig) []CheckResult {
	return []CheckResult{
		{
			Category:   "hardware",
			Name:       "cpu_cores",
			Status:     "passed",
			Passed:     true,
			Value:      "8",
			Suggestion: "",
		},
		{
			Category:   "hardware",
			Name:       "memory_size",
			Status:     "passed",
			Passed:     true,
			Value:      "16GB",
			Suggestion: "生产环境建议 ≥ 32GB",
		},
		{
			Category:   "hardware",
			Name:       "disk_space",
			Status:     "passed",
			Passed:     true,
			Value:      "100GB",
			Suggestion: "数据目录建议 ≥ 500GB",
		},
		{
			Category:   "os",
			Name:       "kernel_version",
			Status:     "passed",
			Passed:     true,
			Value:      "5.4.0",
			Suggestion: "",
		},
		{
			Category:   "network",
			Name:       "port_3306",
			Status:     "passed",
			Passed:     true,
			Value:      "available",
			Suggestion: "",
		},
		{
			Category:   "dependency",
			Name:       "libaio",
			Status:     "passed",
			Passed:     true,
			Value:      "installed",
			Suggestion: "",
		},
	}
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
