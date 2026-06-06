package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

// DataMigrationController 用于"存储后端"切换:
//   - 旧 JSON 文件 -> SQLite (一次启动自动 / 手动触发)
//   - SQLite -> MySQL  (手动触发, 把本地 SQLite 数据搬到 MySQL)
type DataMigrationController struct {
	cfg *config.Config
}

func NewDataMigrationController(cfg *config.Config) *DataMigrationController {
	return &DataMigrationController{cfg: cfg}
}

// resolveSQLitePath 统一处理"用户可能没显式配 sqlite_path"的情况
func (c *DataMigrationController) resolveSQLitePath() string {
	if c.cfg.SQLitePath != "" {
		return c.cfg.SQLitePath
	}
	return c.cfg.DataDir + "/dbops.db"
}

// GetStatus 报告当前数据库类型 / 数据量 / 残留 JSON 文件
func (c *DataMigrationController) GetStatus(ctx *gin.Context) {
	sqlitePath := c.resolveSQLitePath()
	db, err := repositories.NewDatabase(c.cfg.DatabaseURL, sqlitePath)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "open db failed", err)
		return
	}
	defer db.Close()

	counts := map[string]int{
		"hosts":                 0,
		"instances":             0,
		"instance_connections":  0,
		"instance_versions":     0,
		"tasks":                 0,
		"backup_policies":       0,
		"backup_records":        0,
		"alert_rules":           0,
		"alert_records":         0,
		"parameter_templates":   0,
		"migration_tasks":       0,
		"cluster_deployments":   0,
		"users":                 0,
	}
	for table := range counts {
		var n int
		if err := db.Pool.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err == nil {
			counts[table] = n
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dialect":          string(db.Dialect),
			"sqlite_path":      sqlitePath,
			"mysql_configured": c.cfg.DatabaseURL != "",
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
		utils.InternalServerErrorResponse(ctx, "open db failed", err)
		return
	}
	defer db.Close()

	importer := repositories.NewJSONImporter(c.cfg.DataDir)
	n, err := importer.ImportAll(context.Background(), db)
	if err != nil {
		utils.ErrorResponse(ctx, 500, "import failed", err)
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
		utils.BadRequestResponse(ctx, "mysql DSN is empty; please set database_url in config.yaml first")
		return
	}
	sqlitePath := c.resolveSQLitePath()

	src, err := repositories.NewDatabase("", sqlitePath)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "open sqlite failed", err)
		return
	}
	defer src.Close()

	dst, err := repositories.NewDatabase(c.cfg.DatabaseURL, "")
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "open mysql failed", err)
		return
	}
	defer dst.Close()

	mig := repositories.NewMigrator()
	res, err := mig.Migrate(src, dst)
	if err != nil {
		utils.ErrorResponse(ctx, 500, "migration failed", err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "migration completed in " + time.Duration(res.DurationMs).String(),
		"data":    res,
	})
}
