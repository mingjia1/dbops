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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{}, auditSvc)

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
		ClusterID:     "pxc-ui",
		Name:          "pxc-ui",
		BootstrapNode: BootstrapNode{Host: "10.0.0.11", Port: 3306},
		OtherNodes:    []PXCNode{{Host: "10.0.0.12", Port: 3307}},
		PseudoMode:    true,
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
	require.Equal(t, "bootstrap", primary.Status.Role)
	require.Contains(t, primary.Topology.SlaveIDs, "inst-b")

	replica, err := instRepo.GetByID(ctx, "inst-b")
	require.NoError(t, err)
	require.Equal(t, "pxc-ui", replica.ClusterID)
	require.Equal(t, "secondary", replica.Status.Role)
	require.Equal(t, "inst-a", replica.Topology.MasterID)
}

func TestDeployHARealModeSyncsManagedInstances(t *testing.T) {
	ctx := context.Background()
	payloads := make([]DeployTaskPayload, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		var payload DeployTaskPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads = append(payloads, payload)
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})

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
	createManagedInstance("ha-replica-2", "ha-replica-2", "host-replica", 3308)

	resp, err := service.DeployHA(ctx, DeployHARequest{
		ClusterID:    "ha-real-sync",
		Name:         "ha-real-sync",
		MasterHostID: "host-master",
		ReplicaHosts: []SecondaryNode{
			{Host: agentHost, Port: 3307, AgentPort: agentPort, ServerID: 22, DataDir: "/data/mysql/3307", Basedir: "/opt/mysql"},
			{Host: agentHost, Port: 3308, AgentPort: agentPort},
		},
		MasterPort:     3306,
		MasterServerID: 11,
		MasterDataDir:  "/data/mysql/3306",
		MasterBasedir:  "/opt/mysql",
		MySQLUser:      "root",
		MySQLPassword:  "rootpass",
		ConfigParams: map[string]string{
			"package_url":             "https://repo.example/mysql.tar.gz",
			"package_checksum":        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"max_connections":         "512",
			"innodb_buffer_pool_size": "2G",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	require.Len(t, payloads, 3)
	require.Equal(t, "single", payloads[0].Config["deploy_mode"])
	require.Equal(t, "ha-replica", payloads[1].Config["deploy_mode"])
	require.Equal(t, "ha-replica", payloads[2].Config["deploy_mode"])
	require.Equal(t, "127.0.0.1", payloads[0].Config["host"])
	require.Equal(t, "127.0.0.1", payloads[1].Config["slave_host"])
	require.Equal(t, float64(11), payloads[0].Config["server_id"])
	require.Equal(t, "/data/mysql/3306", payloads[0].Config["data_dir"])
	require.Equal(t, "/opt/mysql", payloads[0].Config["basedir"])
	require.Equal(t, "https://repo.example/mysql.tar.gz", payloads[0].Config["package_url"])
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", payloads[0].Config["checksum"])
	require.Equal(t, float64(22), payloads[1].Config["server_id"])
	require.Equal(t, "/data/mysql/3307", payloads[1].Config["data_dir"])
	require.Equal(t, map[string]interface{}{"innodb_buffer_pool_size": "2G", "max_connections": "512"}, payloads[0].Config["mysql_config"])
	master, err := instRepo.GetByID(ctx, "ha-master")
	require.NoError(t, err)
	require.Equal(t, "ha-real-sync", master.ClusterID)
	require.Equal(t, "master", master.Status.Role)
	require.Contains(t, master.Topology.SlaveIDs, "ha-replica")
	require.Contains(t, master.Topology.SlaveIDs, "ha-replica-2")
	replica, err := instRepo.GetByID(ctx, "ha-replica")
	require.NoError(t, err)
	require.Equal(t, "ha-real-sync", replica.ClusterID)
	require.Equal(t, "replica", replica.Status.Role)
	require.Equal(t, "ha-master", replica.Topology.MasterID)
	replica2, err := instRepo.GetByID(ctx, "ha-replica-2")
	require.NoError(t, err)
	require.Equal(t, "ha-real-sync", replica2.ClusterID)
	require.Equal(t, "replica", replica2.Status.Role)
	require.Equal(t, "ha-master", replica2.Topology.MasterID)
}

func TestDestroyClusterWithNoManagedInstancesDestroysMetadata(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "deploy-auditor-002")
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	backupSvc := &BackupService{}
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{}, auditSvc)
	service.SetBackupService(backupSvc)

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "destroy-audit-cluster",
		ClusterType: "ha",
		Name:        "destroy-audit-cluster",
		Status:      "completed",
	}))

	resp, err := service.DestroyCluster(ctx, "destroy-audit-cluster")

	require.NoError(t, err)
	require.Equal(t, "destroyed", resp.Status)
	require.Contains(t, resp.Message, "no managed instances")
	logs, err := auditRepo.ListByResource(context.Background(), "cluster_deployment", "destroy-audit-cluster", 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "destroy_cluster", logs[0].Operation)
	require.Equal(t, "destroy", logs[0].Action)
	require.Equal(t, "success", logs[0].Result)
}

func TestDestroyClusterResolvesDeploymentByName(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "deploy-auditor-003")
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	backupSvc := &BackupService{}
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{})
	service.SetBackupService(backupSvc)

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "destroy-by-name-id",
		ClusterType: "ha",
		Name:        "ha57-for-mha-20260610-v1",
		Status:      "completed",
	}))

	resp, err := service.DestroyCluster(ctx, "ha57-for-mha-20260610-v1")

	require.NoError(t, err)
	require.Equal(t, "destroy-by-name-id", resp.DeploymentID)
	require.Equal(t, "destroyed", resp.Status)
	updated, err := clusterRepo.GetByID(ctx, "destroy-by-name-id")
	require.NoError(t, err)
	require.Equal(t, "destroyed", updated.Status)
}

func TestDestroyClusterRequiresBackupBeforeRemoval(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "destroy-user")
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{})

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "ha-cluster-001",
		ClusterType: "ha",
		Name:        "ha-test",
		Status:      "completed",
	}))
	hostID := "host-001"
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: hostID, Name: "host-001", Address: "10.1.1.10", SSHPort: 22, SSHUser: "root", AgentPort: 9090}))
	require.NoError(t, instRepo.Create(ctx, &models.Instance{
		ID:        "inst-001",
		Name:      "master",
		ClusterID: "ha-cluster-001",
		HostID:    &hostID,
	}))
	require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: "inst-001",
		Host:       "10.1.1.10",
		Port:       3306,
	}))

	_, err := service.DestroyCluster(ctx, "ha-cluster-001")

	require.Error(t, err)
	var destroyErr *ClusterDestroyOperationError
	require.ErrorAs(t, err, &destroyErr)
	require.Equal(t, "ha-cluster-001", destroyErr.ClusterID)
	require.Equal(t, "inst-001", destroyErr.InstanceID)
	require.Contains(t, err.Error(), "backup service is required")
}

func TestClearDestroyedClusterManagementMarksInstancesStopped(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{})

	require.NoError(t, instRepo.Create(ctx, &models.Instance{
		ID:        "destroyed-node",
		Name:      "destroyed-node",
		ClusterID: "destroyed-cluster",
	}))
	require.NoError(t, instRepo.UpsertStatus(ctx, "destroyed-node", &models.InstanceStatus{
		RunStatus:           "running",
		HealthStatus:        "healthy",
		Role:                "master",
		ReplicationStatus:   "mha",
		SecondsBehindMaster: 0,
	}))
	require.NoError(t, instRepo.UpsertTopology(ctx, "destroyed-node", &models.InstanceTopology{
		ClusterID:       "destroyed-cluster",
		ReplicationMode: "mha",
	}))

	require.NoError(t, service.clearDestroyedClusterManagement(ctx, "destroyed-cluster"))

	inst, err := instRepo.GetByID(ctx, "destroyed-node")
	require.NoError(t, err)
	require.Empty(t, inst.ClusterID)
	require.Equal(t, "stopped", inst.Status.RunStatus)
	require.Equal(t, "offline", inst.Status.HealthStatus)
	require.Empty(t, inst.Status.Role)
	require.Empty(t, inst.Topology.ClusterID)
	require.Empty(t, inst.Topology.ReplicationMode)
}

func TestListDeploymentsIncludesManagedNodes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, newTestAgentClient(), config.ClusterDefaults{})

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
		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		configMap, _ := payload["config"].(map[string]interface{})
		if configMap["deploy_mode"] == "mha" {
			_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"mha-error","status":"error","progress":100,"message":"agent deploy error"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"mha-prep","status":"completed","progress":100,"message":"mysql deployed"}}`))
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})

	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)

	hostID := "mha-master"
	// Create host entries for SSH password resolution
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-manager", Name: "mha-manager", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: agentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-master", Name: "mha-master", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: agentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-slave", Name: "mha-slave", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: agentPort}))

	// Create managed instances for management sync
	require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: "mha-master-inst", Name: "mha-master", HostID: &hostID}))
	require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: "mha-master-inst", Host: "10.0.0.31", Port: 3306, Username: "root", PasswordEncrypted: password,
	}))
	require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: "mha-slave-inst", Name: "mha-slave", HostID: &hostID}))
	require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: "mha-slave-inst", Host: "10.0.0.32", Port: 3306, Username: "root", PasswordEncrypted: password,
	}))

	service.SetEncryptionKey("test-encryption-key")

	resp, err := service.DeployCluster(ctx, UniversalClusterDeployRequest{
		ClusterID:   "mha-error-cluster",
		Name:        "mha-error-cluster",
		ClusterType: "mha",
		Mode:        "real",
		MySQL:       MySQLDeployOptions{User: "root", Password: "Root#2026"},
		Replication: ReplicationOptions{User: "repl", Password: "Repl#2026"},
		Nodes: []ClusterDeployNode{
			{HostID: "mha-manager", Host: agentHost, AgentPort: agentPort, MySQLPort: 3306, Role: "manager"},
			{HostID: "mha-master", Host: agentHost, AgentPort: agentPort, MySQLPort: 3306, Role: "master"},
			{HostID: "mha-slave", Host: agentHost, AgentPort: agentPort, MySQLPort: 3307, Role: "replica"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "failed", resp.Status)
	require.Contains(t, resp.Message, "agent deploy error")
	deployment, err := clusterRepo.GetByID(ctx, "mha-error-cluster")
	require.NoError(t, err)
	require.Equal(t, "failed", deployment.Status)
}

func TestDeployMHACreatesManagedMySQLInstancesWithoutManager(t *testing.T) {
	ctx := context.Background()
	payloads := make([]DeployTaskPayload, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		var payload DeployTaskPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads = append(payloads, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"mha-ok","status":"completed","progress":100,"message":"ok"}}`))
	}))
	defer server.Close()
	agentHost, agentPort := splitTestServerHostPort(t, server.URL)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	service.SetEncryptionKey("test-encryption-key")

	sshPassword, err := utils.Encrypt("sshpass", "test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-manager-host", Name: "mha-manager-host", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: sshPassword, AgentPort: agentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-master-host", Name: "mha-master-host", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: sshPassword, AgentPort: agentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mha-replica-host", Name: "mha-replica-host", Address: agentHost, SSHPort: 22, SSHUser: "root", SSHCredential: sshPassword, AgentPort: agentPort}))

	resp, err := service.DeployCluster(ctx, UniversalClusterDeployRequest{
		ClusterID:   "mha-three-node",
		Name:        "mha-three-node",
		ClusterType: "mha",
		Mode:        "real",
		MySQL:       MySQLDeployOptions{User: "root", Password: "Root#2026"},
		Replication: ReplicationOptions{User: "repl", Password: "Repl#2026"},
		Custom:      map[string]interface{}{"ssh_user": "root", "vip_interface": "eth1"},
		Nodes: []ClusterDeployNode{
			{HostID: "mha-manager-host", Host: agentHost, AgentPort: agentPort, MySQLPort: 3306, Role: "manager"},
			{HostID: "mha-master-host", Host: agentHost, AgentPort: agentPort, MySQLPort: 3306, Role: "master", ServerID: 11, DataDir: "/data/mysql/3306"},
			{HostID: "mha-replica-host", Host: agentHost, AgentPort: agentPort, MySQLPort: 3307, Role: "replica", ServerID: 12, DataDir: "/data/mysql/3307"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	require.Len(t, payloads, 3)
	var managerPayload DeployTaskPayload
	for _, payload := range payloads {
		if payload.Config["deploy_mode"] == "mha" {
			managerPayload = payload
			break
		}
	}
	require.Equal(t, "mha", managerPayload.Config["deploy_mode"])
	require.Equal(t, "root", managerPayload.Config["ssh_user"])
	require.Equal(t, "eth1", managerPayload.Config["vip_interface"])
	require.Equal(t, []interface{}{agentHost}, managerPayload.Config["slave_hosts"])
	require.Equal(t, []interface{}{float64(3307)}, managerPayload.Config["slave_ports"])
	sshPasswords, ok := managerPayload.Config["ssh_passwords"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "sshpass", sshPasswords[agentHost])

	instances, err := instRepo.ListByClusterID(ctx, "mha-three-node")
	require.NoError(t, err)
	require.Len(t, instances, 2)

	var masterID, replicaID string
	for _, inst := range instances {
		managed, err := instRepo.GetByID(ctx, inst.ID)
		require.NoError(t, err)
		conn, err := instRepo.GetConnection(ctx, inst.ID)
		require.NoError(t, err)
		require.Equal(t, "root", conn.Username)
		plain, err := utils.Decrypt(conn.PasswordEncrypted, "test-encryption-key")
		require.NoError(t, err)
		require.Equal(t, "Root#2026", plain)
		switch conn.Host {
		case agentHost:
			if conn.Port == 3307 {
				replicaID = inst.ID
				require.Equal(t, "replica", managed.Status.Role)
				continue
			}
			masterID = inst.ID
			require.Equal(t, "master", managed.Status.Role)
		default:
			t.Fatalf("unexpected managed host %s", conn.Host)
		}
	}
	require.NotEmpty(t, masterID)
	require.NotEmpty(t, replicaID)
	replicaTopology, err := instRepo.GetTopology(ctx, replicaID)
	require.NoError(t, err)
	require.Equal(t, masterID, replicaTopology.MasterID)
	require.Equal(t, "mha", replicaTopology.ReplicationMode)
}

func TestClusterDeployWithoutAgentClientFailsDeployment(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		clusterID   string
		clusterType string
		deploy      func(*ClusterDeployService) (*DeployResponse, error)
	}{
		{
			name:        "ha",
			clusterID:   "ha-no-agent",
			clusterType: "ha",
			deploy: func(service *ClusterDeployService) (*DeployResponse, error) {
				return service.DeployHA(ctx, DeployHARequest{
					ClusterID:    "ha-no-agent",
					Name:         "ha-no-agent",
					MasterHost:   "10.0.0.11",
					ReplicaHosts: []SecondaryNode{{Host: "10.0.0.12", Port: 3307, AgentPort: 9090}},
					MasterPort:   3306,

					MySQLUser:     "root",
					MySQLPassword: "rootpass",
				})
			},
		},
		{
			name:        "mha",
			clusterID:   "mha-no-agent",
			clusterType: "mha",
			deploy: func(service *ClusterDeployService) (*DeployResponse, error) {
				return service.DeployMHA(ctx, DeployMHARequest{
					ClusterID:   "mha-no-agent",
					Name:        "mha-no-agent",
					ManagerHost: "10.0.0.10",
					MasterHost:  "10.0.0.11",
					MasterPort:  3306,
					SlaveHosts:  []SlaveNode{{Host: "10.0.0.12", Port: 3307}},
				})
			},
		},
		{
			name:        "mgr",
			clusterID:   "mgr-no-agent",
			clusterType: "mgr",
			deploy: func(service *ClusterDeployService) (*DeployResponse, error) {
				return service.DeployMGR(ctx, DeployMGRRequest{
					ClusterID:      "mgr-no-agent",
					Name:           "mgr-no-agent",
					PrimaryHost:    "10.0.0.11",
					PrimaryPort:    3306,
					SecondaryHosts: []SecondaryNode{{Host: "10.0.0.12", Port: 3307}},
					MySQLUser:      "root",
					MySQLPassword:  "rootpass",
				})
			},
		},
		{
			name:        "pxc",
			clusterID:   "pxc-no-agent",
			clusterType: "pxc",
			deploy: func(service *ClusterDeployService) (*DeployResponse, error) {
				return service.DeployPXC(ctx, DeployPXCRequest{
					ClusterID:     "pxc-no-agent",
					Name:          "pxc-no-agent",
					BootstrapNode: BootstrapNode{Host: "10.0.0.11", Port: 3306},
					OtherNodes:    []PXCNode{{Host: "10.0.0.12", Port: 3307}},
					MySQLUser:     "root",
					MySQLPassword: "rootpass",
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB()
			hostRepo := repositories.NewHostRepository(db)
			instRepo := repositories.NewInstanceRepository(db)
			clusterRepo := repositories.NewClusterDeployRepository(db)
			service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, nil, config.ClusterDefaults{})
			service.SetEncryptionKey("test-encryption-key")

			// Create generic hosts for MHA SSH credential resolution
			password, err := utils.Encrypt("rootpass", "test-encryption-key")
			require.NoError(t, err)
			require.NoError(t, hostRepo.Create(ctx, &models.Host{
				ID: "generic-mgr", Name: "generic-mgr", Address: "10.0.0.10",
				SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: 9090,
			}))
			require.NoError(t, hostRepo.Create(ctx, &models.Host{
				ID: "generic-mst", Name: "generic-mst", Address: "10.0.0.11",
				SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: 9090,
			}))
			require.NoError(t, hostRepo.Create(ctx, &models.Host{
				ID: "generic-slv", Name: "generic-slv", Address: "10.0.0.12",
				SSHPort: 22, SSHUser: "root", SSHCredential: password, AgentPort: 9090,
			}))

			resp, err := tt.deploy(service)

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tt.clusterID, resp.DeploymentID)
			require.Equal(t, tt.clusterType, resp.ClusterType)
			require.Contains(t, []string{"failed", "partial"}, resp.Status)
			require.Contains(t, resp.Message, "agent client not configured")
			deployment, err := clusterRepo.GetByID(ctx, tt.clusterID)
			require.NoError(t, err)
			require.Contains(t, []string{"failed", "partial"}, deployment.Status)
		})
	}
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-primary-host", Name: "mgr-primary-host", Address: primaryHost, SSHPort: 22, SSHUser: "root", AgentPort: primaryAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-secondary-host", Name: "mgr-secondary-host", Address: secondaryHost, SSHPort: 22, SSHUser: "root", AgentPort: secondaryAgentPort}))
	createClusterDeployManagedInstance(t, ctx, instRepo, "mgr-primary-inst", "mgr-primary-host", primaryHost, 3306)
	createClusterDeployManagedInstance(t, ctx, instRepo, "mgr-secondary-inst", "mgr-secondary-host", secondaryHost, 3307)

	resp, err := service.DeployMGR(ctx, DeployMGRRequest{
		ClusterID:        "mgr-agent-port",
		Name:             "mgr-agent-port",
		PrimaryHost:      primaryHost,
		PrimaryPort:      3306,
		PrimaryAgentPort: primaryAgentPort,
		SecondaryHosts:   []SecondaryNode{{Host: secondaryHost, Port: 3307, AgentPort: secondaryAgentPort}},
		MySQLUser:        "root",
		MySQLPassword:    "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "success", resp.Status)
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-bootstrap-host", Name: "pxc-bootstrap-host", Address: bootstrapHost, SSHPort: 22, SSHUser: "root", AgentPort: bootstrapAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-join-host", Name: "pxc-join-host", Address: joinHost, SSHPort: 22, SSHUser: "root", AgentPort: joinAgentPort}))
	createClusterDeployManagedInstance(t, ctx, instRepo, "pxc-bootstrap-inst", "pxc-bootstrap-host", bootstrapHost, 3306)
	createClusterDeployManagedInstance(t, ctx, instRepo, "pxc-join-inst", "pxc-join-host", joinHost, 3307)

	resp, err := service.DeployPXC(ctx, DeployPXCRequest{
		ClusterID:     "pxc-agent-port",
		Name:          "pxc-agent-port",
		BootstrapNode: BootstrapNode{Host: bootstrapHost, Port: 3306, AgentPort: bootstrapAgentPort},
		OtherNodes:    []PXCNode{{Host: joinHost, Port: 3307, AgentPort: joinAgentPort}},
		MySQLUser:     "root",
		MySQLPassword: "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "success", resp.Status)
	deployment, err := clusterRepo.GetByID(ctx, "pxc-agent-port")
	require.NoError(t, err)
	require.Equal(t, "completed", deployment.Status)
}

func TestDeployMGRCreatesManagedInstancesWhenManagementSyncMissing(t *testing.T) {
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-unsynced-primary", Name: "mgr-unsynced-primary", Address: primaryHost, SSHPort: 22, SSHUser: "root", AgentPort: primaryAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "mgr-unsynced-secondary", Name: "mgr-unsynced-secondary", Address: secondaryHost, SSHPort: 22, SSHUser: "root", AgentPort: secondaryAgentPort}))

	resp, err := service.DeployMGR(ctx, DeployMGRRequest{
		ClusterID:        "mgr-unsynced",
		Name:             "mgr-unsynced",
		PrimaryHost:      primaryHost,
		PrimaryPort:      3306,
		PrimaryAgentPort: primaryAgentPort,
		SecondaryHosts:   []SecondaryNode{{Host: secondaryHost, Port: 3307, AgentPort: secondaryAgentPort}},
		MySQLUser:        "root",
		MySQLPassword:    "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "success", resp.Status)
	status, err := service.GetDeploymentStatus(ctx, "mgr-unsynced")
	require.NoError(t, err)
	require.Equal(t, "completed", status.Status)
	instances, err := instRepo.ListByClusterID(ctx, "mgr-unsynced")
	require.NoError(t, err)
	require.Len(t, instances, 2)
}

func TestDeployPXCCreatesManagedInstancesWhenManagementSyncMissing(t *testing.T) {
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
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, NewAgentClient(""), config.ClusterDefaults{})
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-unsynced-bootstrap", Name: "pxc-unsynced-bootstrap", Address: bootstrapHost, SSHPort: 22, SSHUser: "root", AgentPort: bootstrapAgentPort}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "pxc-unsynced-join", Name: "pxc-unsynced-join", Address: joinHost, SSHPort: 22, SSHUser: "root", AgentPort: joinAgentPort}))

	resp, err := service.DeployPXC(ctx, DeployPXCRequest{
		ClusterID:     "pxc-unsynced",
		Name:          "pxc-unsynced",
		BootstrapNode: BootstrapNode{Host: bootstrapHost, Port: 3306, AgentPort: bootstrapAgentPort},
		OtherNodes:    []PXCNode{{Host: joinHost, Port: 3307, AgentPort: joinAgentPort}},
		MySQLUser:     "root",
		MySQLPassword: "rootpass",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "success", resp.Status)
	status, err := service.GetDeploymentStatus(ctx, "pxc-unsynced")
	require.NoError(t, err)
	require.Equal(t, "completed", status.Status)
	instances, err := instRepo.ListByClusterID(ctx, "pxc-unsynced")
	require.NoError(t, err)
	require.Len(t, instances, 2)
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

func requireStepStatus(t *testing.T, steps []DeployStep, name, status string) {
	t.Helper()
	for _, step := range steps {
		if step.Name == name {
			require.Equal(t, status, step.Status)
			return
		}
	}
	t.Fatalf("step %q not found in %#v", name, steps)
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
