package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEnvironmentCheckService(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())
	assert.NotNil(t, service)
}

func TestEnvironmentCheck_Execute(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())

	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{
			{
				Host:     "192.168.1.100",
				Port:     3306,
				Username: "root",
				Password: "password",
			},
		},
	}

	ctx := context.Background()
	result, err := service.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.CheckID, "check-")
	assert.NotZero(t, result.CreatedAt)
}

func TestEnvironmentCheck_Execute_MultipleHosts(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())

	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{
			{Host: "192.168.1.100", Port: 3306, Username: "root", Password: "pass1"},
			{Host: "192.168.1.101", Port: 3306, Username: "root", Password: "pass2"},
		},
	}

	ctx := context.Background()
	result, err := service.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

func TestEnvironmentCheck_GetByID(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())

	ctx := context.Background()
	result, err := service.GetByID(ctx, "check-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "check-001", result.CheckID)
	assert.Equal(t, "completed", result.Status)
	assert.NotZero(t, result.CreatedAt)
}

func TestEnvironmentCheck_checkHost(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())

	host := HostConfig{
		Host:     "192.168.1.100",
		Port:     3306,
		Username: "root",
		Password: "password",
	}

	results := service.checkHost(host)

	assert.Len(t, results, 6)

	for _, r := range results {
		assert.NotEmpty(t, r.Category)
		assert.NotEmpty(t, r.Name)
		assert.NotEmpty(t, r.Status)
		assert.True(t, r.Passed)
		assert.NotEmpty(t, r.Value)
	}
}

func TestHostConfig_Fields(t *testing.T) {
	host := HostConfig{
		Host:     "192.168.1.100",
		Port:     3306,
		Username: "admin",
		Password: "secret",
	}

	assert.Equal(t, "192.168.1.100", host.Host)
	assert.Equal(t, 3306, host.Port)
	assert.Equal(t, "admin", host.Username)
	assert.NotEmpty(t, host.Password)
}

func TestEnvironmentCheckRequest_Fields(t *testing.T) {
	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{},
	}

	assert.Empty(t, req.Hosts)

	req.Hosts = []HostConfig{{Host: "host1"}, {Host: "host2"}}
	assert.Len(t, req.Hosts, 2)
}

func TestEnvironmentCheckResult_Fields(t *testing.T) {
	result := EnvironmentCheckResult{
		CheckID:   "check-001",
		Status:    "completed",
		Results:   []CheckResult{},
		CreatedAt: time.Now(),
	}

	assert.Equal(t, "check-001", result.CheckID)
	assert.Equal(t, "completed", result.Status)
	assert.NotZero(t, result.CreatedAt)
}

func TestCheckResult_Fields(t *testing.T) {
	check := CheckResult{
		Category:   "hardware",
		Name:       "cpu_cores",
		Status:     "passed",
		Passed:     true,
		Value:      "8",
		Suggestion: "",
	}

	assert.Equal(t, "hardware", check.Category)
	assert.Equal(t, "cpu_cores", check.Name)
	assert.Equal(t, "passed", check.Status)
	assert.True(t, check.Passed)
	assert.Equal(t, "8", check.Value)
}

func TestCheckResult_DifferentCategories(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())
	ctx := context.Background()

	result, err := service.Execute(ctx, EnvironmentCheckRequest{
		Hosts: []HostConfig{{Host: "test-host"}},
	})

	assert.NoError(t, err)

	hardwareCount := 0
	osCount := 0
	networkCount := 0
	dependencyCount := 0
	agentCount := 0

	for _, r := range result.Results {
		switch r.Category {
		case "hardware":
			hardwareCount++
		case "os":
			osCount++
		case "network":
			networkCount++
		case "dependency":
			dependencyCount++
		case "agent":
			agentCount++
		}
	}

	assert.Equal(t, 3, hardwareCount)
	assert.Equal(t, 1, osCount)
	assert.Equal(t, 1, networkCount)
	assert.Equal(t, 1, dependencyCount)
	assert.Equal(t, 1, agentCount)
}

func TestCheckResult_AllPassed(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())
	ctx := context.Background()

	result, err := service.Execute(ctx, EnvironmentCheckRequest{
		Hosts: []HostConfig{{Host: "test-host"}},
	})

	assert.NoError(t, err)

	for _, r := range result.Results {
		if r.Category == "agent" {
			assert.False(t, r.Passed)
		} else {
			assert.True(t, r.Passed)
		}
	}
}

func TestEnvironmentCheck_EmptyHosts(t *testing.T) {
	tctx := context.Background()
	service := NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient())

	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{},
	}

	ctx := context.Background()
	result, err := service.Execute(ctx, req)

	assert.NoError(t, err)
	assert.Empty(t, result.Results)
}
