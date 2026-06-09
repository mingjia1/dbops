package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentStub is a programmable Agent stub used by SwitchService tests.
type agentStub struct {
	server   *httptest.Server
	calls    []agentStubCall
	failPath string
}

type agentStubCall struct {
	Path string
	Body map[string]interface{}
}

func newAgentStub() *agentStub {
	s := &agentStub{}
	mux := http.NewServeMux()

	handler := func() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.calls = append(s.calls, agentStubCall{Path: r.URL.Path, Body: body})
			if s.failPath != "" && strings.HasSuffix(r.URL.Path, s.failPath) {
				http.Error(w, "agent failure", http.StatusInternalServerError)
				return
			}
			resp := map[string]interface{}{
				"code":    200,
				"message": "success",
				"data": map[string]interface{}{
					"task_id":  "stub-task",
					"status":   "completed",
					"progress": 100,
					"message":  "stub ok",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}

	mux.HandleFunc("/agent/tasks/role-promote", handler())
	mux.HandleFunc("/agent/tasks/role-demote", handler())
	mux.HandleFunc("/agent/tasks/role-replica-rebuild", handler())
	mux.HandleFunc("/agent/tasks/role-query", handler())
	mux.HandleFunc("/agent/tasks/cluster-switch", handler())
	mux.HandleFunc("/agent/tasks/deploy", handler())
	mux.HandleFunc("/agent/tasks/backup", handler())

	s.server = httptest.NewServer(mux)
	return s
}

func (s *agentStub) Host() string {
	u, _ := url.Parse(s.server.URL)
	return u.Hostname()
}

func (s *agentStub) Port() int {
	u, _ := url.Parse(s.server.URL)
	port, _ := strconv.Atoi(u.Port())
	return port
}

func (s *agentStub) Close() {
	s.server.Close()
}

// helper: build a SwitchService backed by an httptest agent stub.
func newTestSwitchService(t *testing.T) (*SwitchService, *agentStub, *repositories.HostRepository, *repositories.InstanceRepository, *repositories.ClusterDeployRepository) {
	t.Helper()
	ctx := context.Background()

	stub := newAgentStub()

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	host := &models.Host{
		ID:        "host-001",
		Name:      "test-host",
		Address:   stub.Host(),
		AgentPort: stub.Port(),
		SSHPort:   22,
		SSHUser:   "root",
	}
	require.NoError(t, hostRepo.Create(ctx, host))

	instRepo := repositories.NewInstanceRepository(db)
	hostID := "host-001"
	instRepo.Create(ctx, &models.Instance{
		ID:        "inst-old-master",
		HostID:    &hostID,
		ClusterID: "cluster-1",
		Name:      "old-master",
		Status:    models.InstanceStatus{Role: "master"},
		Topology:  models.InstanceTopology{MasterID: ""},
	})
	instRepo.Create(ctx, &models.Instance{
		ID:        "inst-new-master",
		HostID:    &hostID,
		ClusterID: "cluster-1",
		Name:      "new-master",
		Status:    models.InstanceStatus{Role: "slave"},
		Topology:  models.InstanceTopology{MasterID: "inst-old-master"},
	})
	instRepo.Create(ctx, &models.Instance{
		ID:        "inst-replica-1",
		HostID:    &hostID,
		ClusterID: "cluster-1",
		Name:      "replica-1",
		Status:    models.InstanceStatus{Role: "slave"},
		Topology:  models.InstanceTopology{MasterID: "inst-old-master"},
	})

	clusterRepo := repositories.NewClusterDeployRepository(db)
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "cluster-1", ClusterType: "mha", Name: "mha-cluster"})

	agent := NewAgentClient("")
	service := NewSwitchService(hostRepo, instRepo, clusterRepo, agent, nil)
	return service, stub, hostRepo, instRepo, clusterRepo
}

// --- TDD: SwitchService constructor ---
func TestNewSwitchService(t *testing.T) {
	tctx := context.Background()
	hostRepo := newTestHostRepo(tctx)
	instRepo := newTestInstanceRepo(tctx)
	clusterRepo := repositories.NewClusterDeployRepository(newTestDB())
	agent := newTestAgentClient()

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, agent, nil)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.history)
}

// --- TDD: SwitchRoleWithinCluster happy path MHA promote slave -> master ---
func TestSwitchRoleWithinCluster_PromoteMHASlaveToMaster(t *testing.T) {
	ctx := context.Background()
	svc, stub, _, instRepo, _ := newTestSwitchService(t)
	defer stub.Close()

	// Mark current instance as slave.
	inst, err := instRepo.GetByID(ctx, "inst-new-master")
	require.NoError(t, err)
	inst.Status.Role = "slave"
	inst.Topology.MasterID = "inst-old-master"
	require.NoError(t, instRepo.Update(ctx, inst))

	req := RoleSwitchRequest{
		ClusterID:   "cluster-1",
		InstanceID:  "inst-new-master",
		TargetRole:  "master",
		OldMasterID: "inst-old-master",
	}

	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "mha", result.ClusterType)
	assert.Equal(t, "master", result.NewRole)
	assert.Equal(t, "inst-old-master", result.OldMasterID)
	assert.Equal(t, "inst-new-master", result.NewMasterID)
	assert.Contains(t, result.RebuiltReplicas, "inst-replica-1")
	assert.NotEmpty(t, result.TaskID)

	// Verify history record.
	history, err := svc.ListRoleSwitchHistory(ctx, "cluster-1", 10)
	assert.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "inst-new-master", history[0].InstanceID)
}

// --- TDD: MGR primary promotion ---
func TestSwitchRoleWithinCluster_PromoteMGRSecondaryToPrimary(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "host-gr"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "gr-host"})
	instRepo.Create(ctx, &models.Instance{ID: "gr-1", HostID: &hostID, ClusterID: "gr-cluster", Name: "gr-1", Status: models.InstanceStatus{Role: "primary"}, Topology: models.InstanceTopology{MasterID: ""}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "gr-1", &models.InstanceTopology{MasterID: ""}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "gr-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	instRepo.Create(ctx, &models.Instance{ID: "gr-2", HostID: &hostID, ClusterID: "gr-cluster", Name: "gr-2", Status: models.InstanceStatus{Role: "secondary"}, Topology: models.InstanceTopology{MasterID: "gr-1"}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "gr-2", &models.InstanceTopology{MasterID: "gr-1"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "gr-2", &models.InstanceStatus{Role: "secondary", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "gr-cluster", ClusterType: "mgr", Name: "gr-c"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:   "gr-cluster",
		InstanceID:  "gr-2",
		TargetRole:  "primary",
		OldMasterID: "gr-1",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "mgr", result.ClusterType)
	assert.Equal(t, "primary", result.NewRole)
}

// --- TDD: PXC demote primary -> secondary ---
func TestSwitchRoleWithinCluster_DemotePXCPrimaryToSecondary(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h-pxc"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "pxc-h"})
	instRepo.Create(ctx, &models.Instance{ID: "pxc-1", HostID: &hostID, ClusterID: "pxc-cluster", Name: "pxc-1", Status: models.InstanceStatus{Role: "primary"}, Topology: models.InstanceTopology{MasterID: ""}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "pxc-1", &models.InstanceTopology{MasterID: ""}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "pxc-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "pxc-cluster", ClusterType: "pxc", Name: "pxc-c"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "pxc-cluster",
		InstanceID: "pxc-1",
		TargetRole: "secondary",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "secondary", result.NewRole)
}

// --- TDD: skip when current role == target role ---
func TestSwitchRoleWithinCluster_SkipWhenAlreadyInTargetRole(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "inst-skip", HostID: &hostID, ClusterID: "cluster-1", Name: "skip", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "inst-skip", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "inst-skip", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "cluster-1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "cluster-1",
		InstanceID: "inst-skip",
		TargetRole: "master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "skipped", result.Status)
	assert.Equal(t, "master", result.OldRole)
	assert.Equal(t, "master", result.NewRole)
}

// --- TDD: invalid role for cluster type ---
func TestSwitchRoleWithinCluster_InvalidRoleForCluster(t *testing.T) {
	ctx := context.Background()
	svc, stub, _, _, _ := newTestSwitchService(t)
	defer stub.Close()

	req := RoleSwitchRequest{
		ClusterID:  "cluster-1",
		InstanceID: "inst-new-master",
		TargetRole: "primary", // not valid for MHA
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "not valid for cluster architecture")
}

// --- TDD: cluster not found ---
func TestSwitchRoleWithinCluster_ClusterNotFound(t *testing.T) {
	ctx := context.Background()
	svc, stub, _, _, _ := newTestSwitchService(t)
	defer stub.Close()

	req := RoleSwitchRequest{
		ClusterID:  "nonexistent",
		InstanceID: "inst-new-master",
		TargetRole: "master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "cluster")
}

// --- TDD: instance not in specified cluster ---
func TestSwitchRoleWithinCluster_InstanceNotInCluster(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h-x"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "hx"})
	instRepo.Create(ctx, &models.Instance{ID: "inst-a", HostID: &hostID, ClusterID: "other-cluster", Name: "a"})
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "cluster-1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "cluster-1",
		InstanceID: "inst-a",
		TargetRole: "master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "cluster")
}

// --- TDD: instance not found ---
func TestSwitchRoleWithinCluster_InstanceNotFound(t *testing.T) {
	ctx := context.Background()
	svc, stub, _, _, _ := newTestSwitchService(t)
	defer stub.Close()

	req := RoleSwitchRequest{
		ClusterID:  "cluster-1",
		InstanceID: "nope",
		TargetRole: "master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "instance not found")
}

// --- TDD: instance without host ---
func TestSwitchRoleWithinCluster_InstanceWithoutHost(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "cluster-1", ClusterType: "mha", Name: "c1"})
	instRepo.Create(ctx, &models.Instance{ID: "inst-nohost", ClusterID: "cluster-1", Name: "nohost"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "cluster-1",
		InstanceID: "inst-nohost",
		TargetRole: "master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}

// --- TDD: agent failure during promote ---
func TestSwitchRoleWithinCluster_AgentPromoteFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	stub.failPath = "/role-promote"
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "old-m", HostID: &hostID, ClusterID: "c1", Name: "old", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	instRepo.Create(ctx, &models.Instance{ID: "new-m", HostID: &hostID, ClusterID: "c1", Name: "new", Status: models.InstanceStatus{Role: "slave"}, Topology: models.InstanceTopology{MasterID: "old-m"}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "new-m", &models.InstanceTopology{MasterID: "old-m"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "new-m", &models.InstanceStatus{Role: "slave", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:   "c1",
		InstanceID:  "new-m",
		TargetRole:  "master",
		OldMasterID: "old-m",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "promote instance failed")
}

// --- TDD: agent failure during demote-old-master ---
func TestSwitchRoleWithinCluster_AgentDemoteOldMasterFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	stub.failPath = "/role-demote"
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "old-m", HostID: &hostID, ClusterID: "c1", Name: "old", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	instRepo.Create(ctx, &models.Instance{ID: "new-m", HostID: &hostID, ClusterID: "c1", Name: "new", Status: models.InstanceStatus{Role: "slave"}, Topology: models.InstanceTopology{MasterID: "old-m"}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "new-m", &models.InstanceTopology{MasterID: "old-m"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "new-m", &models.InstanceStatus{Role: "slave", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:   "c1",
		InstanceID:  "new-m",
		TargetRole:  "master",
		OldMasterID: "old-m",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "demote old master")
}

// --- TDD: agent failure during demote (instance is current primary) ---
func TestSwitchRoleWithinCluster_AgentDemoteFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	stub.failPath = "/role-demote"
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "i-1", HostID: &hostID, ClusterID: "c1", Name: "i1", Status: models.InstanceStatus{Role: "primary"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "pxc", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "c1",
		InstanceID: "i-1",
		TargetRole: "secondary",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "demote instance failed")
}

// --- TDD: ListRoleSwitchHistory empty ---
func TestListRoleSwitchHistory_Empty(t *testing.T) {
	ctx := context.Background()
	svc, stub, _, _, _ := newTestSwitchService(t)
	defer stub.Close()

	history, err := svc.ListRoleSwitchHistory(ctx, "cluster-1", 10)
	assert.NoError(t, err)
	assert.Empty(t, history)
}

// --- TDD: ListRoleSwitchHistory records and sorts by recency ---
func TestListRoleSwitchHistory_Limit(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "i-1", HostID: &hostID, ClusterID: "c1", Name: "i1", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	for i := 0; i < 3; i++ {
		_, _ = svc.SwitchRoleWithinCluster(ctx, RoleSwitchRequest{
			ClusterID:  "c1",
			InstanceID: "i-1",
			TargetRole: "slave",
		})
		time.Sleep(time.Millisecond) // ensure StartedAt differs
	}

	history, err := svc.ListRoleSwitchHistory(ctx, "c1", 2)
	assert.NoError(t, err)
	assert.Len(t, history, 2)
}

// --- TDD: helpers ---
func TestIsValidRoleForCluster(t *testing.T) {
	cases := []struct {
		clusterType string
		role        string
		want        bool
	}{
		{"mha", "master", true},
		{"mha", "slave", true},
		{"mha", "replica", true},
		{"mha", "primary", false},
		{"mgr", "primary", true},
		{"mgr", "secondary", true},
		{"mgr", "primary_master", true},
		{"mgr", "master", false},
		{"pxc", "primary", true},
		{"pxc", "secondary", true},
		{"pxc", "donor", true},
		{"pxc", "joiner", true},
		{"pxc", "master", false},
		{"unknown", "primary", false},
		{"", "primary", false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s", c.clusterType, c.role), func(t *testing.T) {
			assert.Equal(t, c.want, isValidRoleForCluster(c.clusterType, c.role))
		})
	}
}

func TestIsPrimaryRole(t *testing.T) {
	assert.True(t, isPrimaryRole("master"))
	assert.True(t, isPrimaryRole("primary"))
	assert.True(t, isPrimaryRole("primary_master"))
	assert.False(t, isPrimaryRole("slave"))
	assert.False(t, isPrimaryRole("secondary"))
	assert.False(t, isPrimaryRole("replica"))
	assert.False(t, isPrimaryRole(""))
}

func TestReplicaRoleFor(t *testing.T) {
	assert.Equal(t, "slave", replicaRoleFor("mha"))
	assert.Equal(t, "secondary", replicaRoleFor("mgr"))
	assert.Equal(t, "secondary", replicaRoleFor("pxc"))
	assert.Equal(t, "replica", replicaRoleFor("unknown"))
	assert.Equal(t, "replica", replicaRoleFor(""))
}

// --- TDD: types / structs ---
func TestRoleSwitchRequest_Fields(t *testing.T) {
	r := RoleSwitchRequest{
		ClusterID:       "c-1",
		InstanceID:      "i-1",
		TargetRole:      "master",
		OldMasterID:     "old",
		ReplicationUser: "repl",
		ReplicationPass: "secret",
		Force:           true,
		RequireApproval: false,
		GracePeriodSec:  5,
	}
	assert.Equal(t, "c-1", r.ClusterID)
	assert.Equal(t, "i-1", r.InstanceID)
	assert.Equal(t, "master", r.TargetRole)
	assert.Equal(t, "old", r.OldMasterID)
	assert.True(t, r.Force)
	assert.Equal(t, 5, r.GracePeriodSec)
}

func TestRoleSwitchResult_Fields(t *testing.T) {
	now := time.Now()
	r := RoleSwitchResult{
		TaskID:          "task-1",
		ClusterID:       "c-1",
		ClusterType:     "mha",
		InstanceID:      "i-1",
		InstanceHost:    "1.2.3.4:9090",
		OldRole:         "slave",
		NewRole:         "master",
		OldMasterID:     "old",
		NewMasterID:     "i-1",
		RebuiltReplicas: []string{"r-1", "r-2"},
		Status:          "completed",
		Message:         "ok",
		StartedAt:       now,
		CompletedAt:     now.Add(time.Second),
	}
	assert.NotEmpty(t, r.TaskID)
	assert.Equal(t, "mha", r.ClusterType)
	assert.Len(t, r.RebuiltReplicas, 2)
}

func TestRoleSwitchRecord_Fields(t *testing.T) {
	now := time.Now()
	r := RoleSwitchRecord{
		ID:          "rec-1",
		ClusterID:   "c-1",
		ClusterType: "mha",
		Status:      "completed",
		StartedAt:   now,
		CompletedAt: now,
	}
	assert.Equal(t, "rec-1", r.ID)
	assert.Equal(t, "c-1", r.ClusterID)
}

// --- TDD: demote old master (MHA primary, no slaves) ---
func TestSwitchRoleWithinCluster_DemoteOnlyPrimary(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "mha-master", HostID: &hostID, ClusterID: "c1", Name: "master", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "mha-master", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "mha-master", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "c1",
		InstanceID: "mha-master",
		TargetRole: "slave",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "slave", result.NewRole)
}

// --- TDD: no old master id given; uses inst.Topology.MasterID ---
func TestSwitchRoleWithinCluster_UsesTopologyMasterID(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	instRepo.Create(ctx, &models.Instance{ID: "old-m", HostID: &hostID, ClusterID: "c1", Name: "old-m", Status: models.InstanceStatus{Role: "master"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "old-m", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	instRepo.Create(ctx, &models.Instance{ID: "new-m", HostID: &hostID, ClusterID: "c1", Name: "new-m", Status: models.InstanceStatus{Role: "slave"}, Topology: models.InstanceTopology{MasterID: "old-m"}})
	require.NoError(t, instRepo.UpsertTopology(ctx, "new-m", &models.InstanceTopology{MasterID: "old-m"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "new-m", &models.InstanceStatus{Role: "slave", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "c1",
		InstanceID: "new-m",
		TargetRole: "master",
		// OldMasterID not set
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "old-m", result.OldMasterID)
}

// --- TDD: ambiguous old master id (current instance is primary AND target is primary, different ids) ---
func TestSwitchRoleWithinCluster_AmbiguousOldMaster(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "h"
	hostRepo.Create(ctx, &models.Host{ID: hostID, Address: stub.Host(), AgentPort: stub.Port(), SSHPort: 22, SSHUser: "root", Name: "h1"})
	// Use MGR where "primary" and "primary_master" are both valid primaries
	instRepo.Create(ctx, &models.Instance{ID: "i-1", HostID: &hostID, ClusterID: "c1", Name: "i1", Status: models.InstanceStatus{Role: "primary"}})
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "i-1", &models.InstanceStatus{Role: "primary", HealthStatus: "healthy"}))
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mgr", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:   "c1",
		InstanceID:  "i-1",
		TargetRole:  "primary_master", // different string, but also a primary role
		OldMasterID: "different-master",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "ambiguous old_master_id")
	history, historyErr := svc.ListRoleSwitchHistory(ctx, "c1", 10)
	require.NoError(t, historyErr)
	require.Len(t, history, 1)
	assert.Equal(t, "failed", history[0].Status)
}

// --- TDD: nil pointer for inst.HostID returns error ---
func TestSwitchRoleWithinCluster_NilHostID(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})
	instRepo.Create(ctx, &models.Instance{ID: "no-host", ClusterID: "c1", Name: "nohost"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "c1",
		InstanceID: "no-host",
		TargetRole: "slave",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}

// --- TDD: host repo missing host ---
func TestSwitchRoleWithinCluster_HostRepoMissing(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	stub := newAgentStub()
	defer stub.Close()
	hostID := "missing"
	instRepo.Create(ctx, &models.Instance{ID: "i-1", HostID: &hostID, ClusterID: "c1", Name: "i1"})
	clusterRepo.Create(ctx, &models.ClusterDeployment{ID: "c1", ClusterType: "mha", Name: "c1"})

	svc := NewSwitchService(hostRepo, instRepo, clusterRepo, NewAgentClient(""), nil)
	req := RoleSwitchRequest{
		ClusterID:  "c1",
		InstanceID: "i-1",
		TargetRole: "slave",
	}
	result, err := svc.SwitchRoleWithinCluster(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}
