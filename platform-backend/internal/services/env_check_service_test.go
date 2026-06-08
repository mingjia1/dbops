package services

import (
	"context"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEnvCheckKey = "test-encryption-key"

func newTestEnvCheckCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func newTestEnvCheckService(tctx context.Context) *EnvironmentCheckService {
	return NewEnvironmentCheckService(newTestHostRepo(tctx), newTestAgentClient(), testEnvCheckKey)
}

func TestNewEnvironmentCheckService(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)
	assert.NotNil(t, service)
}

func TestEnvironmentCheck_Execute(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)

	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{
			{Host: "127.0.0.1", Port: 1, Username: "root", Password: "password"},
		},
	}

	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()
	result, err := service.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.CheckID)
	assert.NotZero(t, result.CreatedAt)
}

func TestEnvironmentCheck_Execute_MultipleHosts(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)

	req := EnvironmentCheckRequest{
		Hosts: []HostConfig{
			{Host: "127.0.0.1", Port: 1, Username: "root", Password: "pass1"},
			{Host: "127.0.0.1", Port: 1, Username: "root", Password: "pass2"},
		},
	}

	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()
	result, err := service.Execute(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

func TestEnvironmentCheck_ResolveHostIDsUsesStoredCredential(t *testing.T) {
	ctx := context.Background()
	repo := newTestHostRepo(ctx)
	credential, err := utils.Encrypt("stored-password", testEnvCheckKey)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &models.Host{
		ID:            "host-with-credential",
		Name:          "host-with-credential",
		Address:       "10.0.0.8",
		SSHPort:       2222,
		SSHUser:       "dbadmin",
		SSHAuthMethod: "password",
		SSHCredential: credential,
		AgentPort:     9090,
		OSType:        "linux",
	}))

	service := NewEnvironmentCheckService(repo, newTestAgentClient(), testEnvCheckKey)
	hosts, err := service.resolveHosts(ctx, EnvironmentCheckRequest{
		HostIDs: []string{"host-with-credential"},
	})

	require.NoError(t, err)
	require.Len(t, hosts, 1)
	assert.Equal(t, "10.0.0.8", hosts[0].Host)
	assert.Equal(t, 2222, hosts[0].Port)
	assert.Equal(t, "dbadmin", hosts[0].Username)
	assert.Equal(t, "stored-password", hosts[0].Password)
}

func TestEnvironmentCheck_GetByID(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)

	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()
	result, err := service.GetByID(ctx, "check-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "check-001", result.CheckID)
	assert.Equal(t, "completed", result.Status)
}

func TestEnvironmentCheck_checkHost(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)

	host := HostConfig{Host: "127.0.0.1", Port: 1, Username: "root", Password: "password"}
	results := service.checkHost(host)

	assert.Len(t, results, 6)
	for _, r := range results {
		assert.NotEmpty(t, r.Category)
		assert.NotEmpty(t, r.Name)
		assert.NotEmpty(t, r.Status)
		assert.NotEmpty(t, r.Value)
	}
}

func TestHostConfig_Fields(t *testing.T) {
	host := HostConfig{Host: "192.168.1.100", Port: 3306, Username: "admin", Password: "secret"}
	assert.Equal(t, "192.168.1.100", host.Host)
	assert.Equal(t, 3306, host.Port)
	assert.Equal(t, "admin", host.Username)
	assert.NotEmpty(t, host.Password)
}

func TestEnvironmentCheckRequest_Fields(t *testing.T) {
	req := EnvironmentCheckRequest{Hosts: []HostConfig{}}
	assert.Empty(t, req.Hosts)

	req.Hosts = []HostConfig{{Host: "host1"}, {Host: "host2"}}
	assert.Len(t, req.Hosts, 2)

	req.HostIDs = []string{"host-1", "host-2"}
	assert.Len(t, req.HostIDs, 2)
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
	service := newTestEnvCheckService(tctx)
	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()

	result, err := service.Execute(ctx, EnvironmentCheckRequest{
		Hosts: []HostConfig{{Host: "127.0.0.1", Port: 1, Username: "u", Password: "p"}},
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
	service := newTestEnvCheckService(tctx)
	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()

	result, err := service.Execute(ctx, EnvironmentCheckRequest{
		Hosts: []HostConfig{{Host: "127.0.0.1", Port: 1, Username: "u", Password: "p"}},
	})

	assert.NoError(t, err)
	for _, r := range result.Results {
		if r.Category == "agent" {
			assert.False(t, r.Passed)
		} else if r.Name == "port_reachable" {
			assert.False(t, r.Passed)
		} else {
			assert.Equal(t, "unknown", r.Status)
			assert.False(t, r.Passed)
		}
	}
}

func TestEnvironmentCheck_EmptyHosts(t *testing.T) {
	tctx := context.Background()
	service := newTestEnvCheckService(tctx)

	req := EnvironmentCheckRequest{Hosts: []HostConfig{}}
	ctx, cancel := newTestEnvCheckCtx()
	defer cancel()
	result, err := service.Execute(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
}
