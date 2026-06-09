package services

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInstanceDeployTestService(t *testing.T, passwordEncrypted string, agent *agentStub) (*InstanceService, *repositories.AuditLogRepository, string) {
	t.Helper()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))

	hostID := "deploy-host"
	agentPort := 9090
	agentHost := "127.0.0.1"
	if agent != nil {
		agentHost = agent.Host()
		agentPort = agent.Port()
	}
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{
		ID:        hostID,
		Name:      "deploy-host",
		Address:   agentHost,
		AgentPort: agentPort,
		SSHPort:   22,
		SSHUser:   "root",
	}))

	instanceID := "deploy-instance"
	require.NoError(t, instRepo.Create(context.Background(), &models.Instance{
		ID:     instanceID,
		Name:   "deploy-instance",
		HostID: &hostID,
	}))
	require.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        instanceID,
		Host:              "10.1.81.41",
		Port:              3307,
		Username:          "root",
		PasswordEncrypted: passwordEncrypted,
		Basedir:           "/opt/mysql57",
		Datadir:           "/data/mysql/3307",
		OSUser:            "mysql",
	}))

	return NewInstanceService(instRepo, hostRepo, taskRepo, NewAgentClient(""), auditSvc, "test-encryption-key"), auditRepo, instanceID
}

func TestInstanceDeployWritesSuccessAuditLog(t *testing.T) {
	agent := newAgentStub()
	defer agent.Close()
	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	service, auditRepo, instanceID := newInstanceDeployTestService(t, password, agent)
	ctx := context.WithValue(context.Background(), "user_id", "deploy-user")

	result, err := service.Deploy(ctx, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "instance_deploy_task", result.TaskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "deploy-user", logs[0].UserID)
	assert.Equal(t, "deploy_instance", logs[0].Operation)
	assert.Equal(t, "deploy", logs[0].Action)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, instanceID)
}

func TestInstanceDeployWritesFailedAuditLogOnPasswordDecryptFailure(t *testing.T) {
	service, auditRepo, instanceID := newInstanceDeployTestService(t, "not-encrypted", nil)
	ctx := context.WithValue(context.Background(), "user_id", "deploy-user")

	result, err := service.Deploy(ctx, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "instance_deploy_task", result.TaskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "deploy_instance", logs[0].Operation)
	assert.Equal(t, "failed", logs[0].Result)
	assert.Contains(t, logs[0].ErrorMsg, "illegal base64")
}
