package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/config"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

// DataMigrationController 用于"存储后端"切换:
//   - 旧 JSON 文件 -> SQLite (一次启动自动 / 手动触发)
//   - SQLite -> MySQL  (手动触发, 把本地 SQLite 数据搬到 MySQL)
type DataMigrationController struct {
	cfg *config.Config
	db  *repositories.Database // P0: 复用 running backend 的 db, 不开第二个连接
}

func NewDataMigrationController(cfg *config.Config, db *repositories.Database) *DataMigrationController {
	return &DataMigrationController{cfg: cfg, db: db}
}

// resolveSQLitePath 统一处理"用户可能没显式配 sqlite_path"的情况
func (c *DataMigrationController) resolveSQLitePath() string {
	if c.cfg.SQLitePath != "" {
		return c.cfg.SQLitePath
	}
	return c.cfg.DataDir + "/dbops.db"
}

// isConfiguredMySQLDSN: DatabaseURL 非空且形如可用的 go-sql-driver DSN 才算
// "已配置 MySQL". 占位/哨兵值 (如 "sqlite-local") 会让按钮可点但迁移必然失败.
func isConfiguredMySQLDSN(dsn string) bool {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return false
	}
	return strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "tcp(")
}

// listBusinessTables 动态枚举当前库的全部业务表.
// 固定清单会随 schema 演进遗漏新表 (本库 54 张, 原固定清单只覆盖 13 张),
// 统计与展示必须以实际 schema 为准.
func (c *DataMigrationController) listBusinessTables(ctx context.Context, db *repositories.Database) []string {
	var q string
	if db.Dialect == repositories.DialectSQLite {
		q = `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	} else {
		q = `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name`
	}
	rows, err := db.Pool.QueryContext(ctx, q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	tables := make([]string, 0, 32)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}
	return tables
}

// quoteTableIdent 按方言给表名加引号 (表名来自 schema 枚举, 仍按规范转义).
func quoteTableIdent(name string, dialect repositories.Dialect) string {
	if dialect == repositories.DialectMySQL {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
}

// GetStatus 报告当前数据库类型 / 数据量 / 残留 JSON 文件
func (c *DataMigrationController) GetStatus(ctx *gin.Context) {
	// P0: 之前开第二个 db 连接 (repositories.NewDatabase), mode 不一致时
	// 谎报 dialect (例如 backend 跑 SQLite, GetStatus 这次能连 MySQL 返 mysql).
	// 修: 直接复用 running backend 的 db 实例, 拿到的 dialect 与启动日志一致.
	if c.db == nil {
		utils.InternalServerErrorResponse(ctx, "数据库未初始化", nil)
		return
	}
	db := c.db
	sqlitePath := c.resolveSQLitePath()
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 动态枚举全部业务表统计行数, 展示"各表数据量"与"总记录数".
	tables := c.listBusinessTables(dbCtx, db)
	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		var n int
		if err := db.Pool.QueryRowContext(dbCtx,
			"SELECT COUNT(*) FROM "+quoteTableIdent(table, db.Dialect)).Scan(&n); err == nil {
			counts[table] = n
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dialect":          string(db.Dialect),
			"sqlite_path":      sqlitePath,
			"mysql_configured": isConfiguredMySQLDSN(c.cfg.DatabaseURL),
			"row_counts":       counts,
		},
	})
}

// ImportLegacyJSON 把 data/hosts.json / instances.json 一次性导入到 SQLite.
// 仅在 SQLite 模式 + 对应表为空时生效.
func (c *DataMigrationController) ImportLegacyJSON(ctx *gin.Context) {
	sqlitePath := c.resolveSQLitePath()
	db, err := repositories.NewDatabase(c.cfg.DatabaseURL, sqlitePath)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "打开数据库失败", err)
		return
	}
	defer db.Close()

	importer := repositories.NewJSONImporter(c.cfg.DataDir)
	n, err := importer.ImportAll(context.Background(), db)
	if err != nil {
		utils.ErrorResponse(ctx, 500, "导入失败", err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "import completed",
		"data":    gin.H{"imported": n},
	})
}

// MigrateToMySQL 触发 SQLite -> MySQL 全量迁移
func (c *DataMigrationController) MigrateToMySQL(ctx *gin.Context) {
	if c.cfg.DatabaseURL == "" {
		utils.BadRequestResponse(ctx, "MySQL DSN 为空，请先在 config.yaml 中设置 database_url")
		return
	}
	sqlitePath := c.resolveSQLitePath()

	src, err := repositories.NewDatabase("", sqlitePath)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "打开 SQLite 失败", err)
		return
	}
	defer src.Close()

	dst, err := repositories.NewDatabase(c.cfg.DatabaseURL, "")
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "打开 MySQL 失败", err)
		return
	}
	defer dst.Close()

	mig := repositories.NewMigrator()
	res, err := mig.Migrate(src, dst)
	if err != nil {
		utils.ErrorResponse(ctx, 500, "迁移失败", err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "migration completed in " + time.Duration(res.DurationMs).String(),
		"data":    res,
	})
}
