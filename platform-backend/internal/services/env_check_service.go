package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
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

		if s.agentClient == nil {
			result.Results = append(result.Results, CheckResult{
				Category:   "agent",
				Name:       "agent_connectivity",
				Status:     "failed",
				Passed:     false,
				Value:      fmt.Sprintf("%s:%d", host.Host, agentPort),
				Suggestion: "Backend Agent client is not configured; configure DBOPS_AGENT_TOKEN and restart the backend if Agent auth is required",
			})
			continue
		}

		healthResult, err := s.agentClient.ExecuteHealthCheck(ctx, host.Host, agentPort, "")
		if err != nil {
			result.Results = append(result.Results, CheckResult{
				Category:   "agent",
				Name:       "agent_connectivity",
				Status:     "failed",
				Passed:     false,
				Value:      fmt.Sprintf("%s:%d", host.Host, agentPort),
				Suggestion: "Confirm that the Agent is running on the target host (port 9090)",
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
			Suggestion: "Confirm that SSH is reachable from the backend on the configured host port",
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
	items := []agentCheckSpec{
		{category: "os", name: "os_release", key: "os_release", missingSuggestion: "Agent did not collect the OS release"},
		{category: "os", name: "kernel_version", key: "kernel_version", missingSuggestion: "Agent did not collect the kernel version"},
		{category: "hardware", name: "cpu_cores", key: "cpu_cores", missingSuggestion: "Agent did not collect CPU core count"},
		{category: "hardware", name: "memory_size", key: "memory_size", missingSuggestion: "Agent did not collect memory information"},
		{category: "hardware", name: "disk_space", key: "disk_space", missingSuggestion: "Agent did not collect disk information"},
		{category: "dependency", name: "libaio", key: "libaio", missingSuggestion: "Install the libaio dependency", expected: "installed"},
		{category: "os", name: "vm_swappiness", key: "vm_swappiness", missingSuggestion: "Agent did not collect vm.swappiness", max: intPtr(10), recommendation: "Set vm.swappiness to 10 or lower for database hosts"},
		{category: "os", name: "vm_max_map_count", key: "vm_max_map_count", missingSuggestion: "Agent did not collect vm.max_map_count", min: intPtr(262144), recommendation: "Set vm.max_map_count to at least 262144"},
		{category: "os", name: "vm_overcommit_memory", key: "vm_overcommit_memory", missingSuggestion: "Agent did not collect vm.overcommit_memory", allowed: []string{"0", "1"}, recommendation: "Set vm.overcommit_memory to 0 or 1"},
		{category: "os", name: "fs_file_max", key: "fs_file_max", missingSuggestion: "Agent did not collect fs.file-max", min: intPtr(65535), recommendation: "Set fs.file-max to at least 65535"},
		{category: "os", name: "fs_aio_max_nr", key: "fs_aio_max_nr", missingSuggestion: "Agent did not collect fs.aio-max-nr", min: intPtr(1048576), recommendation: "Set fs.aio-max-nr to at least 1048576 for InnoDB async IO"},
		{category: "os", name: "ulimit_nofile", key: "ulimit_nofile", missingSuggestion: "Agent did not collect ulimit -n", min: intPtr(65535), recommendation: "Raise open files limit to at least 65535"},
		{category: "os", name: "net_core_somaxconn", key: "net_core_somaxconn", missingSuggestion: "Agent did not collect net.core.somaxconn", min: intPtr(1024), recommendation: "Set net.core.somaxconn to at least 1024"},
		{category: "os", name: "net_core_netdev_max_backlog", key: "net_core_netdev_max_backlog", missingSuggestion: "Agent did not collect net.core.netdev_max_backlog", min: intPtr(1000), recommendation: "Set net.core.netdev_max_backlog to at least 1000"},
		{category: "os", name: "net_ipv4_tcp_max_syn_backlog", key: "net_ipv4_tcp_max_syn_backlog", missingSuggestion: "Agent did not collect net.ipv4.tcp_max_syn_backlog", min: intPtr(1024), recommendation: "Set net.ipv4.tcp_max_syn_backlog to at least 1024"},
		{category: "os", name: "net_ipv4_tcp_fin_timeout", key: "net_ipv4_tcp_fin_timeout", missingSuggestion: "Agent did not collect net.ipv4.tcp_fin_timeout", max: intPtr(30), recommendation: "Set net.ipv4.tcp_fin_timeout to 30 or lower"},
		{category: "os", name: "net_ipv4_tcp_keepalive_time", key: "net_ipv4_tcp_keepalive_time", missingSuggestion: "Agent did not collect net.ipv4.tcp_keepalive_time", max: intPtr(600), recommendation: "Set net.ipv4.tcp_keepalive_time to 600 or lower"},
		{category: "os", name: "net_ipv4_tcp_tw_reuse", key: "net_ipv4_tcp_tw_reuse", missingSuggestion: "Agent did not collect net.ipv4.tcp_tw_reuse", allowed: []string{"1"}, recommendation: "Enable net.ipv4.tcp_tw_reuse for high-connection workloads"},
		{category: "os", name: "net_ipv4_ip_local_port_range", key: "net_ipv4_ip_local_port_range", missingSuggestion: "Agent did not collect net.ipv4.ip_local_port_range", rangeWidthMin: 10000, recommendation: "Use a wider local port range, for example 10240 65535"},
		{category: "os", name: "transparent_hugepage", key: "transparent_hugepage", missingSuggestion: "Agent did not collect transparent hugepage status", disallowContains: "[always]", recommendation: "Disable transparent hugepage or set it to madvise"},
	}
	out := make([]CheckResult, 0, len(items))
	for _, item := range items {
		value := stringMapValue(data, item.key)
		passed, suggestion := evaluateAgentCheck(value, item)
		status := "passed"
		if !passed {
			status = "failed"
			if value == "" {
				value = "not_collected"
			}
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

type agentCheckSpec struct {
	category          string
	name              string
	key               string
	missingSuggestion string
	recommendation    string
	expected          string
	allowed           []string
	disallowContains  string
	min               *int
	max               *int
	rangeWidthMin     int
}

func evaluateAgentCheck(value string, spec agentCheckSpec) (bool, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "not_found" {
		return false, spec.missingSuggestion
	}
	if spec.expected != "" && !strings.EqualFold(trimmed, spec.expected) {
		return false, fallbackRecommendation(spec, spec.missingSuggestion)
	}
	if len(spec.allowed) > 0 {
		for _, allowed := range spec.allowed {
			if trimmed == allowed {
				return true, ""
			}
		}
		return false, fallbackRecommendation(spec, spec.missingSuggestion)
	}
	if spec.disallowContains != "" && strings.Contains(strings.ToLower(trimmed), strings.ToLower(spec.disallowContains)) {
		return false, fallbackRecommendation(spec, spec.missingSuggestion)
	}
	if spec.rangeWidthMin > 0 {
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return false, fallbackRecommendation(spec, spec.missingSuggestion)
		}
		low, lowErr := strconv.Atoi(fields[0])
		high, highErr := strconv.Atoi(fields[1])
		if lowErr != nil || highErr != nil || high-low < spec.rangeWidthMin {
			return false, fallbackRecommendation(spec, spec.missingSuggestion)
		}
	}
	if spec.min != nil || spec.max != nil {
		n, err := strconv.Atoi(strings.Fields(trimmed)[0])
		if err != nil {
			return false, fallbackRecommendation(spec, spec.missingSuggestion)
		}
		if spec.min != nil && n < *spec.min {
			return false, fallbackRecommendation(spec, spec.missingSuggestion)
		}
		if spec.max != nil && n > *spec.max {
			return false, fallbackRecommendation(spec, spec.missingSuggestion)
		}
	}
	return true, ""
}

func fallbackRecommendation(spec agentCheckSpec, fallback string) string {
	if spec.recommendation != "" {
		return spec.recommendation
	}
	return fallback
}

func intPtr(v int) *int {
	return &v
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
