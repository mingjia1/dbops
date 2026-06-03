package services

import (
	"context"
	"net/http"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

func newTestHostRepo(tctx context.Context) *repositories.HostRepository {
	repo := repositories.NewHostRepository(nil)
	host := &models.Host{
		ID:        "host-001",
		Name:      "test-host",
		Address:   "192.168.1.100",
		AgentPort: 9090,
		SSHPort:   22,
		SSHUser:   "root",
	}
	repo.Create(tctx, host)
	return repo
}

func newTestInstanceRepo(tctx context.Context) *repositories.InstanceRepository {
	repo := repositories.NewInstanceRepository(nil)
	hostID := "host-001"
	inst := &models.Instance{
		ID:     "instance-001",
		HostID: &hostID,
	}
	repo.Create(tctx, inst)
	return repo
}

func newTestAgentClient() *AgentClient {
	return &AgentClient{
		httpClient: &http.Client{},
	}
}

func newTestMigrationRepo() *repositories.MigrationRepository {
	return repositories.NewMigrationRepository(nil)
}

func newTestMigrationService() *MigrationService {
	return NewMigrationService(newTestMigrationRepo(), newTestInstanceRepo(context.Background()), newTestAgentClient())
}
