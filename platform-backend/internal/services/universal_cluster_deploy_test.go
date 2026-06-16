package services

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestUniversalClusterDeployBuildsMGRPlan(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
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
	require.Contains(t, stepIDs(plan.Steps), "bootstrap_node-1")
	require.Contains(t, stepIDs(plan.Steps), "join_node-2")
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
	db := newTestDB()
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

func TestExecuteClusterDeployPlan_PseudoHA(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
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
	require.Equal(t, "slave", replica.Status.Role)
	require.Equal(t, "ha-master", replica.Topology.MasterID)
}

func TestExecuteClusterDeployPlan_RejectsRealMode(t *testing.T) {
	service := NewClusterDeployService(nil, nil, nil, nil, nil, config.ClusterDefaults{})

	plan := &ClusterDeployPlan{
		DeploymentID: "reject-real",
		ClusterType:  "ha",
		Mode:         "real",
	}

	_, err := service.ExecuteClusterDeployPlan(context.Background(), plan, UniversalClusterDeployRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supports pseudo")
}

func stepIDs(steps []PlanStep) []string {
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.ID)
	}
	return out
}
