package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/config"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestUniversalClusterDeployBuildsMGRPlan(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	service := NewClusterDeployService(repositories.NewClusterDeployRepository(db), nil, hostRepo, repositories.NewInstanceRepository(db), nil, config.ClusterDefaults{})

	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-a", Name: "host-a", Address: "10.0.0.11", AgentPort: 19091}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-b", Name: "host-b", Address: "10.0.0.12", AgentPort: 19092}))
	require.NoError(t, hostRepo.Create(ctx, &models.Host{ID: "host-c", Name: "host-c", Address: "10.0.0.13", AgentPort: 19093}))

	plan, err := service.BuildClusterDeployPlan(ctx, UniversalClusterDeployRequest{
		ClusterID:   "mgr-universal",
		ClusterType: "mgr",
		Replication: ReplicationOptions{Mode: "single-primary"},
		Nodes: []ClusterDeployNode{
			{HostID: "host-a", Role: "primary", MySQLPort: 3306, Custom: map[string]interface{}{"local_port": 33061}},
			{HostID: "host-b", Role: "secondary", MySQLPort: 3306, Custom: map[string]interface{}{"local_port": 33062}},
			{HostID: "host-c", Role: "secondary", MySQLPort: 3306, Custom: map[string]interface{}{"local_port": 33063}},
		},
		Custom: map[string]interface{}{"group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	})

	require.NoError(t, err)
	require.Equal(t, "mgr-universal", plan.DeploymentID)
	require.Equal(t, "mgr", plan.ClusterType)
	require.Len(t, plan.Nodes, 3)
	require.Equal(t, "10.0.0.11", plan.Nodes[0].Host)
	require.Equal(t, 19091, plan.Nodes[0].AgentPort)
	require.Contains(t, stepIDs(plan.Steps), "deploy_mgr_node-1")
	require.Contains(t, stepIDs(plan.Steps), "deploy_mgr_node-2")
	require.Contains(t, stepIDs(plan.Steps), "sync_metadata")
}

func TestUniversalClusterDeployRejectsForbiddenMySQLConfig(t *testing.T) {
	service := NewClusterDeployService(nil, nil, nil, nil, nil, config.ClusterDefaults{})

	_, err := service.ValidateClusterDeploy(context.Background(), UniversalClusterDeployRequest{
		ClusterID:   "ha-invalid-config",
		ClusterType: "ha",
		MySQL:       MySQLDeployOptions{Config: map[string]string{"server_id": "12"}},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3306},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "server_id")
}

func TestUniversalClusterDeployValidateOnlyReturnsPlan(t *testing.T) {
	service := NewClusterDeployService(nil, nil, nil, nil, nil, config.ClusterDefaults{})

	resp, err := service.DeployCluster(context.Background(), UniversalClusterDeployRequest{
		ClusterID:   "ha-validate",
		ClusterType: "ha",
		Mode:        "validate_only",
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3307},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "validated", resp.Status)
	require.Len(t, resp.Nodes, 2)
	require.NotEmpty(t, resp.Steps)
}

func TestUniversalClusterDeployPseudoHAReusesMetadataSync(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, nil, config.ClusterDefaults{})

	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	createInstance := func(id, host string, port int) {
		require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: id, Name: id}))
		require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
			InstanceID:        id,
			Host:              host,
			Port:              port,
			Username:          "root",
			PasswordEncrypted: password,
		}))
	}
	createInstance("ha-master", "10.0.0.11", 3306)
	createInstance("ha-replica", "10.0.0.12", 3306)

	resp, err := service.DeployCluster(ctx, UniversalClusterDeployRequest{
		ClusterID:   "ha-pseudo-universal",
		ClusterType: "ha",
		Mode:        "pseudo",
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3306},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	master, err := instRepo.GetByID(ctx, "ha-master")
	require.NoError(t, err)
	require.Equal(t, "ha-pseudo-universal", master.ClusterID)
	require.Equal(t, "master", master.Status.Role)
}

func TestUniversalClusterDeployMapsCustomParametersToHARequest(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "ha-custom",
		Name:        "ha-custom",
		ClusterType: "ha",
		MySQL: MySQLDeployOptions{
			Version:         "8.0.36",
			PackageURL:      "https://repo.example/mysql.tar.gz",
			PackageChecksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Config:          map[string]string{"max_connections": "512"},
		},
		Replication: ReplicationOptions{User: "repl", Password: "replpass"},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306, AgentPort: 19091, DataDir: "/data/mysql/3306", Basedir: "/opt/mysql", ServerID: 11},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3307, AgentPort: 19092, DataDir: "/data/mysql/3307", ServerID: 12},
		},
		Custom: map[string]interface{}{"semi_sync_enabled": true},
	}

	out := universalToHARequest(req)

	require.Equal(t, "/data/mysql/3306", out.MasterDataDir)
	require.Equal(t, "/opt/mysql", out.MasterBasedir)
	require.Equal(t, 11, out.MasterServerID)
	require.Len(t, out.ReplicaHosts, 1)
	require.Equal(t, 12, out.ReplicaHosts[0].ServerID)
	require.Equal(t, "/data/mysql/3307", out.ReplicaHosts[0].DataDir)
	require.Equal(t, "https://repo.example/mysql.tar.gz", out.ConfigParams["package_url"])
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", out.ConfigParams["package_checksum"])
	require.Equal(t, "512", out.ConfigParams["max_connections"])
}

func TestUniversalClusterDeployMapsCustomParametersToMHARequest(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "mha-custom",
		Name:        "mha-custom",
		ClusterType: "mha",
		MySQL:       MySQLDeployOptions{Config: map[string]string{"innodb_buffer_pool_size": "2G"}},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.10", Role: "manager", AgentPort: 19090},
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3307},
		},
		Custom: map[string]interface{}{"vip": "10.0.0.100", "vip_interface": "ens33", "ping_interval": 5, "ssh_user": "dbops"},
	}

	out := universalToMHARequest(req)

	require.Equal(t, "10.0.0.100", out.VIP)
	require.Equal(t, "ens33", out.ConfigParams["vip_interface"])
	require.Equal(t, "5", out.ConfigParams["ping_interval"])
	require.Equal(t, "dbops", out.ConfigParams["ssh_user"])
	require.Equal(t, "2G", out.ConfigParams["innodb_buffer_pool_size"])
}

func TestUniversalClusterDeployBuildsMHAPlanInstallsMySQLBeforeManager(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "mha-full-flow",
		ClusterType: "mha",
		MySQL:       MySQLDeployOptions{User: "root", Password: "rootpass"},
		Replication: ReplicationOptions{User: "repl", Password: "replpass"},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.10", Role: "manager", AgentPort: 19090},
			{Host: "10.0.0.11", Role: "master", MySQLPort: 3306, AgentPort: 19091, DataDir: "/data/mysql/3306", ServerID: 11},
			{Host: "10.0.0.12", Role: "replica", MySQLPort: 3306, AgentPort: 19092, DataDir: "/data/mysql/3306", ServerID: 12},
		},
	}

	steps := buildMHAPlanSteps([]PlanNode{
		{ID: "manager", Host: "10.0.0.10", Role: "manager", AgentPort: 19090, MySQLPort: 3306},
		{ID: "master", Host: "10.0.0.11", Role: "master", AgentPort: 19091, MySQLPort: 3306, DataDir: "/data/mysql/3306", ServerID: 11},
		{ID: "replica", Host: "10.0.0.12", Role: "replica", AgentPort: 19092, MySQLPort: 3306, DataDir: "/data/mysql/3306", ServerID: 12},
	}, req)

	require.Contains(t, stepIDs(steps), "bootstrap_master")
	require.Contains(t, stepIDs(steps), "join_replica")
	require.Contains(t, stepIDs(steps), "configure_master_master")
	require.Contains(t, stepIDs(steps), "configure_replica_replica")
	require.Contains(t, stepIDs(steps), "deploy_mha_manager")

	masterStep := findStepByID(steps, "bootstrap_master")
	replicaStep := findStepByID(steps, "join_replica")
	configureMasterStep := findStepByID(steps, "configure_master_master")
	configureReplicaStep := findStepByID(steps, "configure_replica_replica")
	managerStep := findStepByID(steps, "deploy_mha_manager")
	require.NotNil(t, masterStep)
	require.NotNil(t, replicaStep)
	require.NotNil(t, configureMasterStep)
	require.NotNil(t, configureReplicaStep)
	require.NotNil(t, managerStep)
	require.Equal(t, "single", masterStep.Config["deploy_mode"])
	require.Equal(t, "single", replicaStep.Config["deploy_mode"])
	require.Equal(t, "ha-master", configureMasterStep.Config["deploy_mode"])
	require.Equal(t, "ha-replica", configureReplicaStep.Config["deploy_mode"])
	require.Equal(t, 3306, masterStep.Config["port"])
	require.Equal(t, 3306, replicaStep.Config["port"])
	require.Equal(t, 11, configureMasterStep.Config["server_id"])
	require.Equal(t, 12, configureReplicaStep.Config["server_id"])
	require.Equal(t, []string{"bootstrap_master"}, configureMasterStep.DependsOn)
	require.Equal(t, []string{"configure_master_master"}, replicaStep.DependsOn)
	require.Equal(t, []string{"join_replica"}, configureReplicaStep.DependsOn)
	require.Equal(t, []string{"prepare_ssh"}, managerStep.DependsOn)
	require.Equal(t, []string{"configure_replica_replica"}, findStepByID(steps, "prepare_ssh").DependsOn)
}

func TestUniversalClusterDeployMapsCustomParametersToMGRRequest(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "mgr-custom",
		Name:        "mgr-custom",
		ClusterType: "mgr",
		Replication: ReplicationOptions{User: "mgrrepl", Password: "mgrpass", Mode: "single-primary"},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "primary", MySQLPort: 3306, AgentPort: 19091, ServerID: 101, Custom: map[string]interface{}{"local_port": 33161}},
			{Host: "10.0.0.12", Role: "secondary", MySQLPort: 3306, AgentPort: 19092, ServerID: 102, Custom: map[string]interface{}{"local_port": 33162}},
		},
		Custom: map[string]interface{}{"group_name": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	}

	out := universalToMGRRequest(req)

	require.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", out.ConfigParams["group_name"])
	require.Equal(t, "mgrrepl", out.ConfigParams["replicate_user"])
	require.Equal(t, "mgrpass", out.ConfigParams["replicate_pass"])
	require.Equal(t, "101", out.ConfigParams["primary_server_id"])
	require.Equal(t, "33161", out.ConfigParams["primary_local_port"])
	require.Len(t, out.SecondaryHosts, 1)
	require.Equal(t, 102, out.SecondaryHosts[0].ServerID)
	require.Equal(t, 33162, out.SecondaryHosts[0].LocalPort)
}

func TestUniversalClusterDeployGeneratesRandomMGRGroupName(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "mgr-default",
		Name:        "mgr-default",
		ClusterType: "mgr",
		Replication: ReplicationOptions{Mode: "single-primary"},
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "primary", MySQLPort: 3306},
			{Host: "10.0.0.12", Role: "secondary", MySQLPort: 3306},
		},
	}

	first := universalToMGRRequest(req)
	second := universalToMGRRequest(req)
	firstGroupName, ok := first.ConfigParams["group_name"]
	require.True(t, ok)
	secondGroupName, ok := second.ConfigParams["group_name"]
	require.True(t, ok)

	_, err := uuid.Parse(firstGroupName)
	require.NoError(t, err)
	_, err = uuid.Parse(secondGroupName)
	require.NoError(t, err)
	require.NotEqual(t, firstGroupName, secondGroupName)

	steps := buildMGRPlanSteps([]PlanNode{
		{ID: "primary", Host: "10.0.0.11", Role: "primary", MySQLPort: 3306},
		{ID: "secondary", Host: "10.0.0.12", Role: "secondary", MySQLPort: 3306},
	}, req)
	step := findStepByID(steps, "deploy_mgr_primary")
	require.NotNil(t, step)
	groupName, ok := step.Config["group_name"].(string)
	require.True(t, ok)
	_, err = uuid.Parse(groupName)
	require.NoError(t, err)
}

func TestUniversalClusterDeployMapsCustomParametersToPXCRequest(t *testing.T) {
	req := UniversalClusterDeployRequest{
		ClusterID:   "pxc-custom",
		Name:        "pxc-custom",
		ClusterType: "pxc",
		Nodes: []ClusterDeployNode{
			{Host: "10.0.0.11", Role: "bootstrap", MySQLPort: 3306, AgentPort: 19091, DataDir: "/data/pxc/3306"},
			{Host: "10.0.0.12", Role: "secondary", MySQLPort: 3307, AgentPort: 19092, DataDir: "/data/pxc/3307"},
		},
		Custom: map[string]interface{}{"cluster_name": "pxc-prod", "sst_method": "xtrabackup-v2", "wsrep_sst_port": 4445, "wsrep_ssl_enabled": true},
	}

	out := universalToPXCRequest(req)

	require.Equal(t, "pxc-prod", out.ConfigParams["cluster_name"])
	require.Equal(t, "xtrabackup-v2", out.ConfigParams["sst_method"])
	require.Equal(t, "4445", out.ConfigParams["wsrep_sst_port"])
	require.Equal(t, "true", out.ConfigParams["wsrep_ssl_enabled"])
	require.Equal(t, "/data/pxc/3306", out.BootstrapNode.DataDir)
	require.Equal(t, "/data/pxc/3307", out.OtherNodes[0].DataDir)
}

func TestTypedPXCRequestToUniversalCarriesPXCCustomParameters(t *testing.T) {
	out := TypedPXCRequestToUniversal(DeployPXCRequest{
		ClusterID: "pxc-legacy",
		Name:      "pxc-legacy",
		BootstrapNode: BootstrapNode{
			Host: "10.0.0.11",
			Port: 3306,
		},
		OtherNodes: []PXCNode{{Host: "10.0.0.12", Port: 3307}},
		SSLEnabled: true,
		WSREPPort:  4569,
		ConfigParams: map[string]string{
			"cluster_name":       "pxc-prod",
			"sst_method":         "xtrabackup-v2",
			"wsrep_sst_port":     "4445",
			"wsrep_ssl_enabled":  "false",
			"max_connections":    "300",
			"wsrep_node_address": "malicious",
		},
	})

	require.Equal(t, "pxc-prod", out.Custom["cluster_name"])
	require.Equal(t, "xtrabackup-v2", out.Custom["sst_method"])
	require.Equal(t, 4569, out.Custom["wsrep_port"])
	require.Equal(t, "4445", out.Custom["wsrep_sst_port"])
	require.Equal(t, "false", out.Custom["wsrep_ssl_enabled"])
	require.Equal(t, "300", out.MySQL.Config["max_connections"])
	require.NotContains(t, out.MySQL.Config, "wsrep_node_address")
	require.NoError(t, validateDeployCustomOptions(ClusterTypePXC, out.Custom, out.MySQL.Config, out.Nodes))

	steps := buildPXCPlanSteps([]PlanNode{
		{ID: "bootstrap", Host: "10.0.0.11", Role: "bootstrap", MySQLPort: 3306},
	}, out)
	require.Equal(t, 4569, steps[4].Config["wsrep_port"])
	require.Equal(t, 4445, steps[4].Config["wsrep_sst_port"])
	require.Equal(t, "false", steps[4].Config["wsrep_ssl_enabled"])
}

func TestTypedPXCRequestToUniversalMapsHostIDsAndRuntimeParams(t *testing.T) {
	out := TypedPXCRequestToUniversal(DeployPXCRequest{
		ClusterID:       "pxc-hostids",
		Name:            "pxc-hostids",
		BootstrapHostID: "host-16",
		OtherHostIDs:    []string{"host-17", "host-18"},
		MySQLUser:       "root",
		MySQLPassword:   "Root#2026",
		WSREPPort:       4569,
		ConfigParams: map[string]string{
			"cluster_name":         "pxc-16-17-18",
			"mysql_port":           "24410",
			"data_dir":             "/data/mysql/pxc-24410",
			"wsrep_sst_port":       "4449",
			"wsrep_ssl_enabled":    "false",
			"force":                "true",
			"innodb_log_file_size": "512M",
		},
	})

	require.Len(t, out.Nodes, 3)
	require.Equal(t, "host-16", out.Nodes[0].HostID)
	require.Equal(t, "bootstrap", out.Nodes[0].Role)
	for _, node := range out.Nodes {
		require.Equal(t, 24410, node.MySQLPort)
		require.Equal(t, "/data/mysql/pxc-24410", node.DataDir)
	}
	require.Equal(t, "true", out.Custom["force"])
	require.NoError(t, validateDeployCustomOptions(ClusterTypePXC, out.Custom, out.MySQL.Config, out.Nodes))

	steps := buildPXCPlanSteps([]PlanNode{
		{ID: "node-16", Host: "10.1.81.16", Role: "bootstrap", MySQLPort: out.Nodes[0].MySQLPort, DataDir: out.Nodes[0].DataDir},
	}, out)
	require.Equal(t, 24410, steps[4].Config["mysql_port"])
	require.Equal(t, "/data/mysql/pxc-24410", steps[4].Config["data_dir"])
	require.Equal(t, "true", steps[4].Config["force"])
}

func TestExecuteClusterDeployPlan_PseudoHA(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, hostRepo, instRepo, nil, config.ClusterDefaults{})

	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	createInstance := func(id, host string, port int) {
		require.NoError(t, instRepo.Create(ctx, &models.Instance{ID: id, Name: id}))
		require.NoError(t, instRepo.CreateConnection(ctx, &models.InstanceConnection{
			InstanceID:        id,
			Host:              host,
			Port:              port,
			Username:          "root",
			PasswordEncrypted: password,
		}))
	}
	createInstance("ha-master", "10.0.0.11", 3306)
	createInstance("ha-replica", "10.0.0.12", 3307)

	plan := &ClusterDeployPlan{
		DeploymentID: "execute-plan-ha",
		ClusterType:  "ha",
		Mode:         "pseudo",
		Nodes: []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "master", AgentPort: 9090},
			{ID: "node-2", Host: "10.0.0.12", MySQLPort: 3307, Role: "replica", AgentPort: 9090},
		},
		Steps: buildUniversalPlanSteps("ha", []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "master"},
			{ID: "node-2", Host: "10.0.0.12", MySQLPort: 3307, Role: "replica"},
		}, UniversalClusterDeployRequest{
			ClusterID: "execute-plan-ha", ClusterType: "ha",
			Replication: ReplicationOptions{User: "repl", Password: "replpass"},
			MySQL:       MySQLDeployOptions{User: "root", Password: "rootpass"},
		}),
	}

	req := UniversalClusterDeployRequest{
		ClusterID:   "execute-plan-ha",
		ClusterType: "ha",
		Mode:        "pseudo",
		Name:        "execute-plan-ha",
		Nodes:       []ClusterDeployNode{{Host: "10.0.0.11", Role: "master", MySQLPort: 3306}, {Host: "10.0.0.12", Role: "replica", MySQLPort: 3307}},
	}

	resp, err := service.ExecuteClusterDeployPlan(ctx, plan, req)
	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	require.Equal(t, "execute-plan-ha", resp.DeploymentID)

	// Verify deployment record was persisted
	dep, err := clusterRepo.GetByID(ctx, "execute-plan-ha")
	require.NoError(t, err)
	require.Equal(t, "completed", dep.Status)
	require.NotEmpty(t, dep.PlanJSON)
	require.NotEmpty(t, dep.RequestJSON)

	// Verify instances were synced
	master, err := instRepo.GetByID(ctx, "ha-master")
	require.NoError(t, err)
	require.Equal(t, "execute-plan-ha", master.ClusterID)
	require.Equal(t, "master", master.Status.Role)

	replica, err := instRepo.GetByID(ctx, "ha-replica")
	require.NoError(t, err)
	require.Equal(t, "execute-plan-ha", replica.ClusterID)
	require.Equal(t, "replica", replica.Status.Role)
	require.Equal(t, "ha-master", replica.Topology.MasterID)
}

func TestExecuteClusterDeployPlan_RealModeFailsWhenNoAgent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, nil, nil, nil, config.ClusterDefaults{})

	plan := &ClusterDeployPlan{
		DeploymentID: "real-no-agent",
		ClusterType:  "ha",
		Mode:         "real",
		Nodes: []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "master", AgentPort: 9090},
		},
		Steps: []PlanStep{
			{ID: "validate_input", Name: "Validate", Type: "validate"},
			{ID: "bootstrap_node-1", Name: "Deploy master", Type: "bootstrap",
				TargetNode: "node-1", AgentPath: "/agent/tasks/deploy",
				Config: map[string]interface{}{"deploy_mode": "ha-master"}},
		},
	}

	req := UniversalClusterDeployRequest{
		ClusterID:   "real-no-agent",
		ClusterType: "ha",
		Mode:        "real",
		Name:        "real-no-agent",
		Nodes:       []ClusterDeployNode{{Host: "10.0.0.11", Role: "master", MySQLPort: 3306}},
	}

	resp, err := service.ExecuteClusterDeployPlan(ctx, plan, req)
	require.NoError(t, err) // Returns a partial response, not an error
	require.Equal(t, "failed", resp.Status)
	require.Contains(t, resp.Message, "agent client not configured")
}

func TestExecuteClusterDeployPlan_PseudoModeCreatesMissingManagedInstances(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	// No instances created — management sync will fail
	service := NewClusterDeployService(clusterRepo, nil, nil, instRepo, nil, config.ClusterDefaults{})

	plan := &ClusterDeployPlan{
		DeploymentID: "pseudo-sync-fail",
		ClusterType:  "mgr",
		Mode:         "pseudo",
		Nodes: []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "primary", AgentPort: 9090},
			{ID: "node-2", Host: "10.0.0.12", MySQLPort: 3307, Role: "secondary", AgentPort: 9090},
		},
		Steps: buildUniversalPlanSteps("mgr", []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "primary"},
			{ID: "node-2", Host: "10.0.0.12", MySQLPort: 3307, Role: "secondary"},
		}, UniversalClusterDeployRequest{
			ClusterID: "pseudo-sync-fail", ClusterType: "mgr",
			Replication: ReplicationOptions{User: "repl", Password: "replpass"},
			MySQL:       MySQLDeployOptions{User: "root", Password: "rootpass"},
		}),
	}

	req := UniversalClusterDeployRequest{
		ClusterID:   "pseudo-sync-fail",
		ClusterType: "mgr",
		Mode:        "pseudo",
		Name:        "pseudo-sync-fail",
		Nodes:       []ClusterDeployNode{{Host: "10.0.0.11", Role: "primary", MySQLPort: 3306}, {Host: "10.0.0.12", Role: "secondary", MySQLPort: 3307}},
	}

	resp, err := service.ExecuteClusterDeployPlan(ctx, plan, req)
	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	instances, err := instRepo.ListByClusterID(ctx, "pseudo-sync-fail")
	require.NoError(t, err)
	require.Len(t, instances, 2)
}

func TestGetDeployPlan_ReturnsDeserializedPlan(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, nil, nil, nil, config.ClusterDefaults{})

	// Create a deployment with plan_json populated
	plan := &ClusterDeployPlan{
		DeploymentID: "plan-test-001",
		ClusterType:  "ha",
		Mode:         "pseudo",
		Nodes: []PlanNode{
			{ID: "node-1", Host: "10.0.0.11", MySQLPort: 3306, Role: "master", AgentPort: 9090, ServerID: 1},
			{ID: "node-2", Host: "10.0.0.12", MySQLPort: 3307, Role: "replica", AgentPort: 9090, ServerID: 2},
		},
		Steps: []PlanStep{
			{ID: "step-1", Name: "validate_input", Type: "validate", TargetNode: ""},
			{ID: "step-2", Name: "deploy_master", Type: "bootstrap", TargetNode: "node-1"},
			{ID: "step-3", Name: "deploy_replica", Type: "join", TargetNode: "node-2"},
			{ID: "step-4", Name: "verify", Type: "verify", TargetNode: ""},
		},
	}
	planJSON := mustMarshalJSON(plan)

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "plan-test-001",
		ClusterType: "ha",
		Name:        "plan-test-001",
		Status:      "completed",
		PlanJSON:    planJSON,
	}))

	// Call GetDeployPlan
	result, err := service.GetDeployPlan(ctx, "plan-test-001")
	require.NoError(t, err)
	require.Equal(t, "plan-test-001", result.DeploymentID)
	require.Equal(t, "ha", result.ClusterType)
	require.Len(t, result.Nodes, 2)
	require.Equal(t, "10.0.0.11", result.Nodes[0].Host)
	require.Equal(t, "master", result.Nodes[0].Role)
	require.Equal(t, 1, result.Nodes[0].ServerID)
	require.Equal(t, "10.0.0.12", result.Nodes[1].Host)
	require.Equal(t, 2, result.Nodes[1].ServerID)
	require.Len(t, result.Steps, 4)
	require.Equal(t, "validate_input", result.Steps[0].Name)
	require.Equal(t, "deploy_master", result.Steps[1].Name)
}

func TestGetDeployPlan_ReturnsErrorWhenEmpty(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	clusterRepo := repositories.NewClusterDeployRepository(db)
	service := NewClusterDeployService(clusterRepo, nil, nil, nil, nil, config.ClusterDefaults{})

	require.NoError(t, clusterRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "plan-test-empty",
		ClusterType: "ha",
		Name:        "plan-test-empty",
		Status:      "pending",
		PlanJSON:    "",
	}))

	_, err := service.GetDeployPlan(ctx, "plan-test-empty")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no plan found")
}

func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func stepIDs(steps []PlanStep) []string {
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.ID)
	}
	return out
}

func findStepByID(steps []PlanStep, id string) *PlanStep {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}
