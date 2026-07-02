package controllers

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

var testDBCounter uint64

// newTestDB 返回一个使用独立 SQLite 文件的 Database, 并已跑完 schema migration.
// 每个测试调用都拿到独立 db, 避免 state leak.
// 每次调用前删除可能残留的旧文件, 保证从空 schema 启动.
func newTestDB(t *testing.T) *repositories.Database {
	t.Helper()
	n := atomic.AddUint64(&testDBCounter, 1)
	dir := filepath.Join(os.TempDir(), "dbops-controller-test")
	_ = os.MkdirAll(dir, 0o755)
	dbPath := filepath.Join(dir, "test-"+strconv.FormatUint(n, 10)+".db")
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	db, err := repositories.NewDatabaseWithMode("", dbPath, "sqlite")
	if err != nil {
		t.Fatal("failed to open test sqlite: ", err.Error())
	}
	if err := repositories.RunMigrations(context.Background(), db); err != nil {
		t.Fatal("failed to run migrations: ", err.Error())
	}
	return db
}

// parsePagination P1: 统一 controller 层 limit/offset 解析, 避免每个 List handler
// 重复写同样的 if-else; 边界 / 负数 / 极大值 集中处理.
//
// 规则:
//   - limit 缺省 20, 范围 [1, 200], 超出夹紧 (不返 400, 跟旧版行为一致)
//   - offset 缺省 0, 必须 >= 0
//   - 非数字 fallback 到缺省 (而不是 400, 防止爬虫把后端搞挂)
func parsePagination(c *gin.Context) (int, int) {
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}
	return limit, offset
}
