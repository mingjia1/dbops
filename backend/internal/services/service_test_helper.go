package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

var testDBCounter uint64

// newTestDB 返回一个使用独立 SQLite 文件的 Database, 并已跑完 schema migration.
// 每个测试调用都拿到独立 db, 避免 state leak.
// 调用者负责在测试结束时 db.Close().
// 每次调用前删除可能残留的旧文件, 保证从空 schema 启动.
func newTestDB() *repositories.Database {
	n := atomic.AddUint64(&testDBCounter, 1)
	dir := filepath.Join(os.TempDir(), "dbops-test")
	_ = os.MkdirAll(dir, 0o755)
	dbPath := filepath.Join(dir, "test-"+strconv.FormatUint(n, 10)+".db")
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	db, err := repositories.NewDatabaseWithMode("", dbPath, "sqlite")
	if err != nil {
		panic("failed to open test sqlite: " + err.Error())
	}
	if err := repositories.RunMigrations(context.Background(), db); err != nil {
		panic("failed to run migrations: " + err.Error())
	}
	return db
}

func newTestHostRepo(tctx context.Context) *repositories.HostRepository {
	repo := repositories.NewHostRepository(newTestDB())
	host := &models.Host{
		ID:        "host-001",
		Name:      "test-host",
		Address:   "192.168.1.100",
		AgentPort: 9090,
		SSHPort:   22,
		SSHUser:   "root",
	}
	_ = repo.Create(tctx, host)
	return repo
}

func newTestInstanceRepo(tctx context.Context) *repositories.InstanceRepository {
	repo := repositories.NewInstanceRepository(newTestDB())
	hostID := "host-001"
	inst := &models.Instance{
		ID:     "instance-001",
		HostID: &hostID,
	}
	_ = repo.Create(tctx, inst)
	return repo
}

func newTestAgentClient() *AgentClient {
	return &AgentClient{
		httpClient: &http.Client{},
	}
}

func newTestMigrationRepo() *repositories.MigrationRepository {
	return repositories.NewMigrationRepository(newTestDB())
}

func newTestMigrationService() *MigrationService {
	return NewMigrationService(newTestMigrationRepo(), newTestInstanceRepo(context.Background()), nil, newTestAgentClient())
}
