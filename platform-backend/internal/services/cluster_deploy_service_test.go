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
}
