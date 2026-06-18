package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceCreateFillsPackageURLFromVersionCatalog(t *testing.T) {
	db := newTestDB()
	instRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewInstanceService(instRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")

	instance, err := service.Create(context.Background(), CreateInstanceRequest{
		Name:      "versioned-instance",
		Host:      "10.1.81.41",
		Port:      3307,
		Username:  "root",
		Password:  "rootpass",
		VersionID: "mysql-5.7.44",
	})

	require.NoError(t, err)
	conn, err := instRepo.GetConnection(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.Equal(t, "mysql-5.7.44", conn.VersionID)
	assert.Contains(t, conn.PackageURL, "mysql-5.7.44")
}

func TestInstanceUpdateConnectionInfoPreservesPasswordWhenBlank(t *testing.T) {
	db := newTestDB()
	instRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewInstanceService(instRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")

	instance, err := service.Create(context.Background(), CreateInstanceRequest{
		Name:     "editable-instance",
		Host:     "10.1.81.16",
		Port:     24410,
		Username: "root",
		Password: "Root#2026",
	})
	require.NoError(t, err)
	before, err := instRepo.GetConnection(context.Background(), instance.ID)
	require.NoError(t, err)

	updated, err := service.Update(context.Background(), instance.ID, UpdateInstanceRequest{
		Name:       "editable-instance",
		Host:       "10.1.81.17",
		Port:       24411,
		Username:   "admin",
		Password:   "",
		SSLEnabled: boolPtr(true),
		Basedir:    "/opt/dbops-pxc/usr",
		Datadir:    "/data/pxc/24411",
		OSUser:     "mysql",
	})

	require.NoError(t, err)
	assert.Equal(t, "10.1.81.17", updated.Connection.Host)
	assert.Equal(t, 24411, updated.Connection.Port)
	assert.Equal(t, "admin", updated.Connection.Username)
	assert.True(t, updated.Connection.SSLEnabled)
	assert.Equal(t, "/opt/dbops-pxc/usr", updated.Connection.Basedir)
	assert.Equal(t, "/data/pxc/24411", updated.Connection.Datadir)
	after, err := instRepo.GetConnection(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.Equal(t, before.PasswordEncrypted, after.PasswordEncrypted)
	password, err := utils.Decrypt(after.PasswordEncrypted, "test-encryption-key")
	require.NoError(t, err)
	assert.Equal(t, "Root#2026", password)
}

func TestInstanceUpdateConnectionInfoUpdatesPasswordWhenProvided(t *testing.T) {
	db := newTestDB()
	instRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewInstanceService(instRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")

	instance, err := service.Create(context.Background(), CreateInstanceRequest{
		Name:     "password-edit-instance",
		Host:     "10.1.81.16",
		Port:     24410,
		Username: "root",
		Password: "Old#2026",
	})
	require.NoError(t, err)

	_, err = service.Update(context.Background(), instance.ID, UpdateInstanceRequest{
		Name:     "password-edit-instance",
		Password: "Root#2026",
	})

	require.NoError(t, err)
	conn, err := instRepo.GetConnection(context.Background(), instance.ID)
	require.NoError(t, err)
	password, err := utils.Decrypt(conn.PasswordEncrypted, "test-encryption-key")
	require.NoError(t, err)
	assert.Equal(t, "Root#2026", password)
}

func boolPtr(v bool) *bool {
	return &v
}

func TestAgentClientDeployInstanceAddsCatalogPackageWithoutUnverifiedChecksum(t *testing.T) {
	var payload DeployTaskPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "deploy-task",
				"status":   "completed",
				"progress": 100,
				"message":  "ok",
			},
		})
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	instance := &models.Instance{
		ID: "deploy-instance",
		Connection: models.InstanceConnection{
			Host:      "10.1.81.41",
			Port:      3307,
			Username:  "root",
			VersionID: "mysql-5.7.44",
		},
	}
	_, err = NewAgentClient("").DeployInstance(context.Background(), u.Hostname(), port, instance, "deploy-task", "rootpass")

	require.NoError(t, err)
	assert.Contains(t, payload.Config["package_url"], "mysql-5.7.44")
	assert.NotContains(t, payload.Config, "checksum")
}

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

func TestInstanceHealthCheckFailedAgentResultMarksUnhealthy(t *testing.T) {
	var payload DeployTaskPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/health-check", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "health-check-test",
				"status":   "failed",
				"progress": 100,
				"message":  "agent refused connection",
			},
		})
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	agentPort, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	hostID := "health-host"
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{
		ID:        hostID,
		Name:      "health-host",
		Address:   u.Hostname(),
		AgentPort: agentPort,
		SSHPort:   22,
		SSHUser:   "root",
	}))
	instanceID := "health-instance"
	require.NoError(t, instRepo.Create(context.Background(), &models.Instance{
		ID:     instanceID,
		Name:   "health-instance",
		HostID: &hostID,
	}))
	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        instanceID,
		Host:              "10.1.81.41",
		Port:              3306,
		Username:          "root",
		PasswordEncrypted: password,
	}))
	service := NewInstanceService(instRepo, hostRepo, taskRepo, NewAgentClient(""), nil, "test-encryption-key")

	result, err := service.HealthCheck(context.Background(), instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, instanceID, payload.InstanceID)
	assert.Equal(t, "10.1.81.41", payload.Config["target_host"])
	assert.Equal(t, float64(3306), payload.Config["target_port"])
	assert.Equal(t, "root", payload.Config["target_user"])
	assert.Equal(t, "rootpass", payload.Config["target_pass"])
	status, err := instRepo.GetStatus(context.Background(), instanceID)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", status.HealthStatus)
}

func TestResolveAgentMySQLTargetUsesLocalhostForSameManagedHost(t *testing.T) {
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewInstanceService(instRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	hostID := "same-host"
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{
		ID:        hostID,
		Name:      "same-host",
		Address:   "10.1.81.21",
		AgentPort: 9090,
		SSHPort:   22,
		SSHUser:   "root",
	}))
	instance := &models.Instance{ID: "same-instance", HostID: &hostID}
	conn := &models.InstanceConnection{InstanceID: instance.ID, Host: "10.1.81.21", Port: 23306}

	assert.Equal(t, "127.0.0.1", service.resolveAgentMySQLTarget(context.Background(), instance, conn, "10.1.81.21"))
	assert.Equal(t, "10.1.81.21", service.resolveAgentMySQLTarget(context.Background(), instance, conn, "10.1.81.22"))
}

func newInstanceAdminServiceWithoutAgent(t *testing.T) (*InstanceService, string) {
	t.Helper()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	hostID := "admin-host"
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{
		ID:        hostID,
		Name:      "admin-host",
		Address:   "127.0.0.1",
		AgentPort: 9090,
		SSHPort:   22,
		SSHUser:   "root",
	}))
	instanceID := "admin-instance"
	require.NoError(t, instRepo.Create(context.Background(), &models.Instance{
		ID:     instanceID,
		Name:   "admin-instance",
		HostID: &hostID,
	}))
	password, err := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        instanceID,
		Host:              "10.1.81.41",
		Port:              3307,
		Username:          "root",
		PasswordEncrypted: password,
	}))
	return NewInstanceService(instRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key"), instanceID
}

func TestInstanceAdminActionWithoutAgentClientReturnsFailedResult(t *testing.T) {
	service, instanceID := newInstanceAdminServiceWithoutAgent(t)

	result, err := service.AdminAction(context.Background(), instanceID, InstanceAdminRequest{
		Action:   "change_password",
		Username: "root",
		Password: "NewPass#2026",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, 100, result.Progress)
	assert.Contains(t, result.Message, "agent client not configured")
}

func TestValidateInstanceAdminMetadataForServiceControl(t *testing.T) {
	tests := []struct {
		name    string
		conn    *models.InstanceConnection
		req     InstanceAdminRequest
		wantErr string
	}{
		{
			name: "non service control skips instance layout metadata",
			conn: &models.InstanceConnection{},
			req:  InstanceAdminRequest{Action: "change_password"},
		},
		{
			name:    "missing connection metadata",
			conn:    nil,
			req:     InstanceAdminRequest{Action: "service_control", Verb: "status"},
			wantErr: "instance connection metadata is required",
		},
		{
			name:    "status requires datadir",
			conn:    &models.InstanceConnection{Port: 3307},
			req:     InstanceAdminRequest{Action: "service_control", Verb: "status"},
			wantErr: "service control requires instance datadir metadata",
		},
		{
			name: "start requires basedir",
			conn: &models.InstanceConnection{
				Datadir: "/data/mysql/3307",
				Port:    3307,
			},
			req:     InstanceAdminRequest{Action: "service_control", Verb: "start"},
			wantErr: "service control start requires instance basedir metadata",
		},
		{
			name: "restart requires port",
			conn: &models.InstanceConnection{
				Basedir: "/opt/mysql57",
				Datadir: "/data/mysql/3307",
			},
			req:     InstanceAdminRequest{Action: "service_control", Verb: "restart"},
			wantErr: "service control restart requires instance port metadata",
		},
		{
			name: "start accepts complete metadata",
			conn: &models.InstanceConnection{
				Basedir: "/opt/mysql57",
				Datadir: "/data/mysql/3307",
				Port:    3307,
			},
			req: InstanceAdminRequest{Action: "service_control", Verb: " START "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceAdminMetadata(tt.conn, tt.req)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInstanceAdminActionServiceControlMissingDatadirReturnsFailedResult(t *testing.T) {
	service, instanceID := newInstanceAdminServiceWithoutAgent(t)

	result, err := service.AdminAction(context.Background(), instanceID, InstanceAdminRequest{
		Action: "service_control",
		Verb:   "status",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, 100, result.Progress)
	assert.Contains(t, result.Message, "service control requires instance datadir metadata")
}

func TestResolveInstanceConfigPathUsesPXCManagedConfig(t *testing.T) {
	conn := &models.InstanceConnection{
		Port:    24410,
		Datadir: "/data/mysql/pxc-24410",
	}

	assert.Equal(t, "/etc/dbops-pxc/dbops-pxc-24410.cnf", resolveInstanceConfigPath(conn, ""))
	assert.Equal(t, "/etc/dbops-pxc/dbops-pxc-24410.cnf", resolveInstanceConfigPath(conn, "/etc/my.cnf"))
	assert.Equal(t, "/custom/my.cnf", resolveInstanceConfigPath(conn, "/custom/my.cnf"))
}

func TestResolveInstanceConfigPathDetectsMHA(t *testing.T) {
	conn := &models.InstanceConnection{
		Port:    3307,
		Datadir: "/data/mysql/mha-3307",
	}

	assert.Equal(t, "/etc/mha/app1.cnf", resolveInstanceConfigPath(conn, ""))
	assert.Equal(t, "/etc/mha/app1.cnf", resolveInstanceConfigPath(conn, "/etc/my.cnf"))
	assert.Equal(t, "/custom/my.cnf", resolveInstanceConfigPath(conn, "/custom/my.cnf"))
}

func TestResolveInstanceConfigPathDetectsMGR(t *testing.T) {
	conn := &models.InstanceConnection{
		Port:    3306,
		Datadir: "/data/mysql/mgr-3306",
	}

	assert.Equal(t, "/etc/my.cnf", resolveInstanceConfigPath(conn, ""))
	assert.Equal(t, "/etc/my.cnf", resolveInstanceConfigPath(conn, "/etc/my.cnf"))
	assert.Equal(t, "/custom/my.cnf", resolveInstanceConfigPath(conn, "/custom/my.cnf"))
}

func TestResolveInstanceConfigPathReturnsDefaultForStandardInstance(t *testing.T) {
	conn := &models.InstanceConnection{
		Port:    3306,
		Datadir: "/data/mysql/3306",
	}

	assert.Equal(t, "/etc/my.cnf", resolveInstanceConfigPath(conn, ""))
	assert.Equal(t, "/etc/my.cnf", resolveInstanceConfigPath(conn, "/etc/my.cnf"))
}

func TestResolveInstanceConfigPathReturnsDefaultWhenConnIsNil(t *testing.T) {
	assert.Equal(t, "/etc/my.cnf", resolveInstanceConfigPath(nil, ""))
}

func TestBatchUpdatePasswordWithoutAgentClientReturnsFailedRow(t *testing.T) {
	service, _ := newInstanceAdminServiceWithoutAgent(t)

	result, err := service.BatchUpdatePassword(context.Background(), BatchPasswordRequest{
		Host:         "10.1.81.41",
		Ports:        []int{3307},
		Username:     "root",
		NewPassword:  "NewPass#2026",
		UpdateStored: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	data, ok := result.Data.(map[string]interface{})
	require.True(t, ok)
	rows, ok := data["rows"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, rows, 1)
	assert.Equal(t, "failed", rows[0]["status"])
	assert.Contains(t, rows[0]["message"], "agent client not configured")
}

func TestInstanceAdminChangePasswordRejectsWeakPassword(t *testing.T) {
	service, instanceID := newInstanceAdminServiceWithoutAgent(t)

	result, err := service.AdminAction(context.Background(), instanceID, InstanceAdminRequest{
		Action:   "change_password",
		Username: "root",
		Password: "weakpass",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "uppercase")
}

func TestBatchUpdatePasswordRejectsWeakPassword(t *testing.T) {
	service, _ := newInstanceAdminServiceWithoutAgent(t)

	result, err := service.BatchUpdatePassword(context.Background(), BatchPasswordRequest{
		Host:        "10.1.81.41",
		Ports:       []int{3307},
		Username:    "root",
		NewPassword: "weakpass",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "uppercase")
}

func TestDefaultMySQLPasswordForConnection(t *testing.T) {
	assert.Equal(t, "Hcfc@DboOps#2024_57", defaultMySQLPasswordForConnection(&models.InstanceConnection{VersionID: "mysql-5.7.44"}))
	assert.Equal(t, "Hcfc@DboOps#2024_56", defaultMySQLPasswordForConnection(&models.InstanceConnection{Basedir: "/opt/mysql-5.6"}))
	assert.Equal(t, "Hcfc@DboOps#2024_80", defaultMySQLPasswordForConnection(&models.InstanceConnection{VersionID: "mysql-8.0.36"}))
}
