package services

import (
	"context"
	"fmt"
	"net"
	"runtime"
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

// checkHost B4: 之前 6 条检查全写死 Passed:true, 客户被假数据误导.
// 修: TCP 端口可达性用 net.Dial 真探测; CPU/内存/磁盘/内核/依赖 标记 unknown (需要 agent
// 真实采集, 当前 backend 没有 SSH 通道, 不能假报 passed).
func (s *EnvironmentCheckService) checkHost(host HostConfig) []CheckResult {
	results := []CheckResult{}

	// 真探测: TCP 3306 可达
	addr := net.JoinHostPort(host.Host, fmt.Sprintf("%d", host.Port))
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		results = append(results, CheckResult{
			Category:   "network",
			Name:       "port_reachable",
			Status:     "failed",
			Passed:     false,
			Value:      fmt.Sprintf("%s: %v", addr, err),
			Suggestion: "确认 MySQL 已启动且 3306 端口对 backend 可达",
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

	// 硬件/OS/依赖 真探测需要 agent 走 SSH/sysfs, 当前 backend 没有 SSH,
	// 标 unknown 让用户看到 "需要安装 agent 才有真实数据", 不再假报 passed.
	for _, item := range []struct{ cat, name, suggestion string }{
		{"hardware", "cpu_cores", "需要 agent 端采集"},
		{"hardware", "memory_size", "需要 agent 端采集"},
		{"hardware", "disk_space", "需要 agent 端采集"},
		{"os", "kernel_version", "需要 agent 端采集"},
		{"dependency", "libaio", "需要 agent 端采集"},
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

	// 本地 backend 的 runtime 信息 (仅供 debug, 不冒充目标主机)
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
