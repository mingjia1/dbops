package services

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestDeployPXC_PseudoModeResolvesReplicaHostIDsOverEmptyOtherNodes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	ctx = context.WithValue(ctx, "user_id", "deploy-auditor-001")
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{}, auditSvc)

	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-a", Name: "host-a", Address: "10.0.0.11", SSHPort: 22, SSHUser: "root", AgentPort: 9090}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-b", Name: "host-b", Address: "10.0.0.12", SSHPort: 22, SSHUser: "root", AgentPort: 9090}))

	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	createManagedInstance := func(id, name, hostID, host string, port int) {
		require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: id, Name: name, HostID: &hostID}))
		require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
			InstanceID:        id,
			Host:              host,
			Port:              port,
			Username:          "root",
			PasswordEncrypted: password,
		}))
	}
	createManagedInstance("inst-a", "pxc-a", "host-a", "10.0.0.11", 3306)
	createManagedInstance("inst-b", "pxc-b", "host-b", "10.0.0.12", 3307)

	resp, err := service.DeployPXC(ctx, DeployPXCRequest{
		ClusterID:       "pxc-ui",
		Name:            "pxc-ui",
		BootstrapHostID: "host-a",
		OtherHostIDs:    []string{"host-b"},
		BootstrapNode:   BootstrapNode{Port: 3306},
		OtherNodes:      []PXCNode{{Port: 3307}},
		PseudoMode:      true,
	})
	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "cluster_deployment", resp.DeploymentID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "deploy-auditor-001", logs[0].UserID)
	require.Equal(t, "deploy_pxc_cluster", logs[0].Operation)
	require.Equal(t, "success", logs[0].Result)
	require.Contains(t, logs[0].Details, "cluster_type=pxc")

	primary, err := instRepo.GetByID(ctx, "inst-a")
	require.NoError(t, err)
	require.Equal(t, "pxc-ui", primary.ClusterID)
	require.Equal(t, "primary", primary.Status.Role)
	require.Contains(t, primary.Topology.SlaveIDs, "inst-b")

	replica, err := instRepo.GetByID(ctx, "inst-b")
	require.NoError(t, err)
	require.Equal(t, "pxc-ui", replica.ClusterID)
	require.Equal(t, "secondary", replica.Status.Role)
	require.Equal(t, "inst-a", replica.Topology.MasterID)
}

func TestDeployHARealModeSyncsManagedInstances(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"ha-sync","status":"completed","progress":100,"message":"ha deployed"}}`))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	agentHost, agentPortText, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	agentPort, err := strconv.Atoi(agentPortText)
	require.NoError(t, err)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})

	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-master", Name: "host-master", Address: agentHost, SSHPort: 22, SSHUser: "root", AgentPort: agentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-replica", Name: "host-replica", Address: agentHost, SSHPort: 22, SSHUser: "root", AgentPort: agentPort}))
	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	createManagedInstance := func(id, name, hostID string, port int) {
		require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: id, Name: name, HostID: &hostID}))
		require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
			InstanceID:        id,
			Host:              agentHost,
			Port:              port,
			Username:          "root",
			PasswordEncrypted: password,
		}))
	}
	createManagedInstance("ha-master", "ha-master", "host-master", 3306)
	createManagedInstance("ha-replica", "ha-replica", "host-replica", 3307)

	resp, err := service.DeployHA(ctx, DeployHARequest{
		ClusterID:     "ha-real-sync",
		Name:          "ha-real-sync",
		MasterHostID:  "host-master",
		ReplicaHostID: "host-replica",
		MasterPort:    3306,
		ReplicaPort:   3307,
		MySQLUser:     "root",
		MySQLPassword: "rootpass",
	})

	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	master, err := instRepo.GetByID(ctx, "ha-master")
	require.NoError(t, err)
	require.Equal(t, "ha-real-sync", master.ClusterID)
	require.Equal(t, "master", master.Status.Role)
	require.Contains(t, master.Topology.SlaveIDs, "ha-replica")
	replica, err := instRepo.GetByID(ctx, "ha-replica")
	require.NoError(t, err)
	require.Equal(t, "ha-real-sync", replica.ClusterID)
	require.Equal(t, "slave", replica.Status.Role)
	require.Equal(t, "ha-master", replica.Topology.MasterID)
}

func TestDestroyClusterWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "deploy-auditor-002")
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{}, auditSvc)

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "destroy-audit-cluster",
		ClusterType: "ha",
		Name:        "destroy-audit-cluster",
		Status:      "completed",
	}))

	resp, err := service.DestroyCluster(ctx, "destroy-audit-cluster")

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "destroyed", resp.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "cluster_deployment", "destroy-audit-cluster", 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "destroy_cluster", logs[0].Operation)
	require.Equal(t, "destroy", logs[0].Action)
	require.Equal(t, "success", logs[0].Result)

	deployments, err := service.ListDeployments(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	require.Equal(t, "destroy-audit-cluster", deployments[0].DeploymentID)
	require.Equal(t, "destroyed", deployments[0].Status)
}

func TestListDeploymentsIncludesManagedNodes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{})

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "cluster-with-nodes",
		ClusterType: "mgr",
		Name:        "cluster-with-nodes",
		Status:      "completed",
	}))
	hostID := "node-host"
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: hostID, Name: "node-host", Address: "10.0.0.21", SSHPort: 22, SSHUser: "root", AgentPort: 9090}))
	require.NoError(t, instRepo.Create(ctx, &models.Instance{
		ID:        "node-instance",
		Name:      "node-instance",
		ClusterID: "cluster-with-nodes",
		HostID:    &hostID,
	}))
	require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: "node-instance",
		Host:       "10.0.0.21",
		Port:       3306,
		Username:   "root",
	}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "node-instance", &models.InstanceStatus{
		InstanceID:          "node-instance",
		RunStatus:           "running",
		HealthStatus:        "healthy",
		Role:                "primary",
		SecondsBehindMaster: -1,
	}))

	deployments, err := service.ListDeployments(ctx, 10, 0)

	require.NoError(t, err)
	require.Len(t, deployments, 1)
	require.Len(t, deployments[0].Nodes, 1)
	require.Equal(t, "node-instance", deployments[0].Nodes[0].InstanceID)
	require.Equal(t, "10.0.0.21", deployments[0].Nodes[0].Host)
	require.Equal(t, 3306, deployments[0].Nodes[0].Port)
	require.Equal(t, "primary", deployments[0].Nodes[0].Role)
}

func TestNormalizeDeployStatusMapsAgentFailureStatuses(t *testing.T) {
	require.Equal(t, "success", normalizeDeployStatus("completed"))
	require.Equal(t, "success", normalizeDeployStatus("ok"))
	require.Equal(t, "failed", normalizeDeployStatus("error"))
	require.Equal(t, "failed", normalizeDeployStatus("timeout"))
	require.Equal(t, "failed", normalizeDeployStatus("unhealthy"))
	require.Equal(t, "partial", normalizeDeployStatus("partial_success"))
}

func TestDeployMHAAgentErrorStatusIsPersistedAsFailed(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"mha-error","status":"error","progress":100,"message":"agent deploy error"}}`))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	agentHost, agentPortText, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	agentPort, err := strconv.Atoi(agentPortText)
	require.NoError(t, err)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})

	resp, err := service.DeployMHA(ctx, DeployMHARequest{
		ClusterID:        "mha-error-cluster",
		Name:             "mha-error-cluster",
		ManagerHost:      agentHost,
		ManagerAgentPort: agentPort,
		MasterHost:       "10.0.0.31",
		MasterPort:       3306,
		SlaveHosts:       []SlaveNode{{Host: "10.0.0.32", Port: 3306}},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "failed", resp.Status)
	require.Contains(t, resp.Message, "agent deploy error")
	deployment, err := clusterRepo.GetByID(ctx, "mha-error-cluster")
	require.NoError(t, err)
	require.Equal(t, "failed", deployment.Status)
}

func TestDeployMGRUsesResolvedAgentPorts(t *testing.T) {
	ctx := context.Background()
	primaryServer := httptest.NewServer(clusterDeployOKHandler(t))
	defer primaryServer.Close()
	secondaryServer := httptest.NewServer(clusterDeployOKHandler(t))
	defer secondaryServer.Close()
	primaryHost, primaryAgentPort := splitTestServerHostPort(t, primaryServer.URL)
	secondaryHost, secondaryAgentPort := splitTestServerHostPort(t, secondaryServer.URL)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-primary-host", Name: "mgr-primary-host", Address: primaryHost, SSHPort: 22, SSHUser: "root", AgentPort: primaryAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-secondary-host", Name: "mgr-secondary-host", Address: secondaryHost, SSHPort: 22, SSHUser: "root", AgentPort: secondaryAgentPort}))
	createClusterDeployManagedInstance(t, ctx, instRepo, "mgr-primary-inst", "mgr-primary-host", primaryHost, 3306)
	createClusterDeployManagedInstance(t, ctx, instRepo, "mgr-secondary-inst", "mgr-secondary-host", secondaryHost, 3307)

	resp, err := service.DeployMGR(ctx, DeployMGRRequest{
		ClusterID:        "mgr-agent-port",
		Name:             "mgr-agent-port",
		PrimaryHostID:    "mgr-primary-host",
		SecondaryHostIDs: []string{"mgr-secondary-host"},
		PrimaryPort:      3306,
		ReplicaPort:      3307,
		MySQLUser:        "root",
		MySQLPassword:    "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "completed", resp.Status)
	deployment, err := clusterRepo.GetByID(ctx, "mgr-agent-port")
	require.NoError(t, err)
	require.Equal(t, "completed", deployment.Status)
}

func TestDeployPXCUsesResolvedAgentPorts(t *testing.T) {
	ctx := context.Background()
	bootstrapServer := httptest.NewServer(clusterDeployOKHandler(t))
	defer bootstrapServer.Close()
	joinServer := httptest.NewServer(clusterDeployOKHandler(t))
	defer joinServer.Close()
	bootstrapHost, bootstrapAgentPort := splitTestServerHostPort(t, bootstrapServer.URL)
	joinHost, joinAgentPort := splitTestServerHostPort(t, joinServer.URL)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-bootstrap-host", Name: "pxc-bootstrap-host", Address: bootstrapHost, SSHPort: 22, SSHUser: "root", AgentPort: bootstrapAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-join-host", Name: "pxc-join-host", Address: joinHost, SSHPort: 22, SSHUser: "root", AgentPort: joinAgentPort}))
	createClusterDeployManagedInstance(t, ctx, instRepo, "pxc-bootstrap-inst", "pxc-bootstrap-host", bootstrapHost, 3306)
	createClusterDeployManagedInstance(t, ctx, instRepo, "pxc-join-inst", "pxc-join-host", joinHost, 3307)

	resp, err := service.DeployPXC(ctx, DeployPXCRequest{
		ClusterID:       "pxc-agent-port",
		Name:            "pxc-agent-port",
		BootstrapHostID: "pxc-bootstrap-host",
		OtherHostIDs:    []string{"pxc-join-host"},
		BootstrapNode:   BootstrapNode{Port: 3306},
		OtherNodes:      []PXCNode{{Port: 3307}},
		MySQLUser:       "root",
		MySQLPassword:   "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "completed", resp.Status)
	deployment, err := clusterRepo.GetByID(ctx, "pxc-agent-port")
	require.NoError(t, err)
	require.Equal(t, "completed", deployment.Status)
}

func clusterDeployOKHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "cluster-deploy-test",
				"status":   "completed",
				"progress": 100,
				"message":  "deployed",
			},
		})
	}
}

func splitTestServerHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	host, portText, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return host, port
}

func createClusterDeployManagedInstance(t *testing.T, ctx context.Context, repo *repositories.InstanceRepository, id, hostID, host string, port int) {
	t.Helper()
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: id, Name: id, HostID: &hostID}))
	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, repo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID:        id,
		Host:              host,
		Port:              port,
		Username:          "root",
		PasswordEncrypted: password,
	}))
}
