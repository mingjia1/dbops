package controllers

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

var testDBCounter uint64

// newTestDB 返回一个使用独立 SQLite 文件的 Database, 并已跑完 schema migration.
// 每个测试调用都拿到独立 db, 避免 state leak.
// 每次调用前删除可能残留的旧文件, 保证从空 schema 启动.
func newTestDB() *repositories.Database {
	n := atomic.AddUint64(&testDBCounter, 1)
	dir := filepath.Join(os.TempDir(), "dbops-controller-test")
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
