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
	return NewDatabaseWithMode(mysqlDSN, sqlitePath, "auto")
}

// NewDatabaseWithMode P0-6: 显式选择存储后端行为, 杜绝静默降级.
//   mode = "mysql"  -> 强制连 MySQL, 连不上直接 fail (不静默回退).
//   mode = "sqlite" -> 强制使用 SQLite, 忽略 mysqlDSN.
//   mode = "auto"   -> 优先 MySQL, 失败回退 SQLite (旧行为, 仅 dev 友好).
func NewDatabaseWithMode(mysqlDSN, sqlitePath, mode string) (*Database, error) {
	mode = normalizeMode(mode)

	if mode == "sqlite" {
		return openSQLite(sqlitePath)
	}

	if mysqlDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := sql.Open("mysql", mysqlDSN)
		if err == nil {
			pool.SetMaxOpenConns(20)
			pool.SetMaxIdleConns(5)
			pool.SetConnMaxLifetime(30 * time.Minute)
			pool.SetConnMaxIdleTime(5 * time.Minute)
			if pingErr := pool.PingContext(ctx); pingErr == nil {
				return &Database{Pool: pool, Dialect: DialectMySQL}, nil
			} else {
				pool.Close()
				if mode == "mysql" {
					return nil, fmt.Errorf("storage_mode=mysql but MySQL connection failed: %w", pingErr)
				}
			}
		} else if mode == "mysql" {
			return nil, fmt.Errorf("storage_mode=mysql but sql.Open failed: %w", err)
		}
	} else if mode == "mysql" {
		return nil, fmt.Errorf("storage_mode=mysql requires database_url in config.yaml")
	}

	if mode == "mysql" {
		// 走到这里说明 MySQL 模式但 mysqlDSN 为空 — 已在上面 fail.
		return nil, fmt.Errorf("storage_mode=mysql unreachable")
	}

	if sqlitePath == "" {
		return nil, fmt.Errorf("no database available: MySQL connection failed and sqlite path is empty")
	}
	return openSQLite(sqlitePath)
}

func normalizeMode(m string) string {
	switch m {
	case "mysql", "sqlite", "auto", "":
		if m == "" {
			return "auto"
		}
		return m
	default:
		return "auto"
	}
}

func openSQLite(sqlitePath string) (*Database, error) {
	if sqlitePath == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}
	dir := filepath.Dir(sqlitePath)
	if dir != "" && dir != "." {
		_ = mkdirAll(dir)
	}
	dsn := sqlitePath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	pool, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite at %s: %w", sqlitePath, err)
	}
	pool.SetMaxOpenConns(1)
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
