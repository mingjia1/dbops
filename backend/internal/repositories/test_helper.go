package repositories

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

// newRepoTestDB 返回一个使用独立 SQLite 文件的 Database, 并已跑完 schema migration.
// 用于 repositories 包内的 _test.go 文件, 避免和 services/controllers 包的 helper 重复.
// 每个测试调用都拿到独立 db, 避免 state leak.
// 每次调用前删除可能残留的旧文件, 保证从空 schema 启动.
func newRepoTestDB() *Database {
	n := atomic.AddUint64(&repoTestDBCounter, 1)
	dir := filepath.Join(os.TempDir(), "dbops-repo-test")
	_ = os.MkdirAll(dir, 0o755)
	dbPath := filepath.Join(dir, "test-"+strconv.FormatUint(n, 10)+".db")
	// 清掉旧文件, 避免上次测试运行残留的数据 / schema 状态干扰本次断言.
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	db, err := NewDatabaseWithMode("", dbPath, "sqlite")
	if err != nil {
		panic("failed to open test sqlite: " + err.Error())
	}
	if err := RunMigrations(context.Background(), db); err != nil {
		panic("failed to run migrations: " + err.Error())
	}
	return db
}

// seedTestHost 在测试 db 中插入一台主机, 返回 hostID.
// 供 instance 测试使用, 因为 instance.host_id 有外键约束, 必须有 host 行才能 insert.
func seedTestHost(t interface{ Helper() }, db *Database, name string) string {
	repo := NewHostRepository(db)
	host := &models.Host{
		Name:    name,
		Address: "127.0.0.1",
	}
	if err := repo.Create(context.Background(), host); err != nil {
		panic("seedTestHost: " + err.Error())
	}
	return host.ID
}

var repoTestDBCounter uint64

