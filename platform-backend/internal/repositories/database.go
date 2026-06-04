package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// Dialect 区分当前数据库类型, 让 repository 可以写一份同时兼容两端的 SQL
type Dialect string

const (
	DialectMySQL  Dialect = "mysql"
	DialectSQLite Dialect = "sqlite"
)

type Database struct {
	Pool    *sql.DB
	Dialect Dialect
}

// NewDatabase 优先尝试 MySQL, 失败则回退到 SQLite. SQLite 文件位于 cfg.DataDir/dbops.db.
// 这样可以做到:
//   1. 生产环境正常连 MySQL, 走主库
//   2. 离线 / 测试 / 用户没部署 MySQL, 自动落 SQLite, 同样能持久化
func NewDatabase(mysqlDSN, sqlitePath string) (*Database, error) {
	if mysqlDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := sql.Open("mysql", mysqlDSN)
		if err == nil {
			pool.SetMaxOpenConns(20)
			pool.SetMaxIdleConns(5)
			pool.SetConnMaxLifetime(30 * time.Minute)
			pool.SetConnMaxIdleTime(5 * time.Minute)
			if err = pool.PingContext(ctx); err == nil {
				return &Database{Pool: pool, Dialect: DialectMySQL}, nil
			}
			pool.Close()
		}
	}

	if sqlitePath == "" {
		return nil, fmt.Errorf("no database available: MySQL connection failed and sqlite path is empty")
	}

	dir := filepath.Dir(sqlitePath)
	if dir != "" && dir != "." {
		_ = mkdirAll(dir)
	}

	// SQLite DSN: _pragma=... 启用外键 / busy_timeout / WAL
	dsn := sqlitePath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	pool, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite at %s: %w", sqlitePath, err)
	}
	pool.SetMaxOpenConns(1) // SQLite 写串行, 多个连接会引起锁问题
	pool.SetConnMaxLifetime(0)

	if err := pool.Ping(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping sqlite: %w", err)
	}
	return &Database{Pool: pool, Dialect: DialectSQLite}, nil
}

func (db *Database) Close() error {
	if db == nil || db.Pool == nil {
		return nil
	}
	return db.Pool.Close()
}

func (db *Database) HealthCheck(ctx context.Context) error {
	if db == nil || db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if db.Dialect == DialectSQLite {
		return db.Pool.PingContext(ctx)
	}
	return db.Pool.PingContext(ctx)
}

// IsSQLite 简化调用方代码
func (db *Database) IsSQLite() bool { return db != nil && db.Dialect == DialectSQLite }
func (db *Database) IsMySQL() bool  { return db != nil && db.Dialect == DialectMySQL }

// mkdirAll 避免循环依赖, 复用 jsonstore 创建目录
func mkdirAll(dir string) error {
	return osMkdirAll(dir)
}
