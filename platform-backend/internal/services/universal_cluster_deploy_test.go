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
	service := NewClusterDeployService(repositories.NewClusterDeployRepository(db), hostRepo, repositories.NewInstanceRepository(db), nil, config.ClusterDefaults{})

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
	service := NewClusterDeployService(nil, nil, nil, nil, config.ClusterDefaults{})

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
	service := NewClusterDeployService(nil, nil, nil, nil, config.ClusterDefaults{})

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
	service := NewClusterDeployService(clusterRepo, hostRepo, instRepo, nil, config.ClusterDefaults{})

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

func stepIDs(steps []PlanStep) []string {
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.ID)
	}
	return out
}
