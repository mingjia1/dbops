package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Migrator 把当前 db (任意方言) 的数据全量导出, 然后灌到目标 db.
// 用法: SQLite 起步 -> 切到 MySQL 时调用一次, 把本地数据搬过去.
type Migrator struct{}

func NewMigrator() *Migrator { return &Migrator{} }

// MigrateReport 单张表迁移的结果
type MigrateReport struct {
	Table   string `json:"table"`
	Rows    int    `json:"rows"`
	Status  string `json:"status"` // "ok" / "skipped" / "failed"
	Message string `json:"message"`
}

// MigrateResult 总体结果
type MigrateResult struct {
	Tables     []MigrateReport `json:"tables"`
	TotalRows  int             `json:"total_rows"`
	DurationMs int64           `json:"duration_ms"`
	Error      string          `json:"error,omitempty"`
}

// migrations 决定哪些表需要搬, 顺序保证外键先主后从.
// 注意: 我们只搬业务数据, 不搬 schema (target 上由 target 的 RunMigrations 自己建表).
var migrateTables = []string{
	"hosts",
	"instances",
	"instance_connections",
	"instance_versions",
	"instance_configs",
	"instance_statuses",
	"instance_topologies",
	"users",
	"tasks",
	"task_workflows",
	"task_checkpoints",
	"task_logs",
	"backup_policies",
	"backup_records",
	"restore_records",
	"alert_rules",
	"alert_records",
	"parameter_templates",
	"parameter_template_versions",
	"parameter_template_parameters",
	"migration_tasks",
	"cluster_deployments",
	"cluster_credentials",
	"topology_events",
	"deploy_workflows",
}

// Migrate src -> dst. src 和 dst 都必须是已经 Ping 通的 *sql.DB.
// 流程:
//   1. 在 dst 上跑 RunMigrations (确保表存在)
//   2. 对每张表: src 读所有行 -> 构造 INSERT 语句 (复用列顺序) -> dst 写入
//   3. 用 INSERT OR REPLACE / ON DUPLICATE KEY 避免主键冲突 (重复迁移时安全)
func (m *Migrator) Migrate(src, dst *Database) (*MigrateResult, error) {
	if src == nil || src.Pool == nil {
		return nil, fmt.Errorf("source database is nil")
	}
	if dst == nil || dst.Pool == nil {
		return nil, fmt.Errorf("target database is nil")
	}
	if src == dst {
		return nil, fmt.Errorf("source and target are the same database")
	}

	started := time.Now()
	result := &MigrateResult{}

	// 先确保目标库有表
	if err := RunMigrations(context.Background(), dst); err != nil {
		result.Error = "target schema migration failed: " + err.Error()
		return result, err
	}

	for _, table := range migrateTables {
		report := m.migrateTable(context.Background(), src, dst, table)
		result.Tables = append(result.Tables, report)
		if report.Status == "ok" {
			result.TotalRows += report.Rows
		}
	}

	result.DurationMs = time.Since(started).Milliseconds()
	return result, nil
}

func (m *Migrator) migrateTable(ctx context.Context, src, dst *Database, table string) MigrateReport {
	report := MigrateReport{Table: table, Status: "ok"}

	// 检查表是否存在 (两端都要有)
	if !m.tableExists(ctx, src.Pool, table) {
		report.Status = "skipped"
		report.Message = "table not found in source"
		return report
	}
	if !m.tableExists(ctx, dst.Pool, table) {
		report.Status = "skipped"
		report.Message = "table not found in target"
		return report
	}

	// 取列名
	cols, err := m.getColumns(ctx, src.Pool, table)
	if err != nil {
		report.Status = "failed"
		report.Message = "list columns: " + err.Error()
		return report
	}
	if len(cols) == 0 {
		report.Status = "skipped"
		report.Message = "no columns"
		return report
	}

	// 读所有数据
	rows, err := src.Pool.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", quoteJoined(cols, src.Dialect), table))
	if err != nil {
		report.Status = "failed"
		report.Message = "select: " + err.Error()
		return report
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		report.Status = "failed"
		report.Message = "col types: " + err.Error()
		return report
	}

	// 构造 UPSERT 语句: ON DUPLICATE KEY UPDATE id=id (MySQL) / INSERT OR REPLACE (SQLite)
	// 用 "INSERT INTO ... ON CONFLICT/ON DUPLICATE KEY" 风格时, 语法两端不同, 这里用先 DELETE 再 INSERT.
	// 实现: 在 dst 上 BEGIN; 一边读 src 一边写 dst, 用 INSERT OR IGNORE / ON DUPLICATE KEY 防止重复
	tx, err := dst.Pool.BeginTx(ctx, nil)
	if err != nil {
		report.Status = "failed"
		report.Message = "begin tx: " + err.Error()
		return report
	}
	defer tx.Rollback()

	upsertSQL := m.buildUpsertSQL(table, cols, dst.Dialect)
	stmt, err := tx.PrepareContext(ctx, upsertSQL)
	if err != nil {
		report.Status = "failed"
		report.Message = "prepare: " + err.Error()
		return report
	}
	defer stmt.Close()

	values := make([]interface{}, len(cols))
	scanDest := make([]interface{}, len(cols))
	for i := range scanDest {
		scanDest[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			report.Status = "failed"
			report.Message = "scan: " + err.Error()
			return report
		}
		// NULL-safe 转换
		converted := convertRowValues(values, colTypes, dst.Dialect)
		if _, err := stmt.ExecContext(ctx, converted...); err != nil {
			// 主键冲突时跳过 (重复迁移安全)
			if isDuplicateError(err) {
				continue
			}
			report.Status = "failed"
			report.Message = fmt.Sprintf("insert row %d: %v", report.Rows, err)
			return report
		}
		report.Rows++
	}
	if err := rows.Err(); err != nil {
		report.Status = "failed"
		report.Message = "rows iter: " + err.Error()
		return report
	}

	if err := tx.Commit(); err != nil {
		report.Status = "failed"
		report.Message = "commit: " + err.Error()
		return report
	}
	return report
}

func (m *Migrator) tableExists(ctx context.Context, db *sql.DB, table string) bool {
	var n int
	q := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`
	if err := db.QueryRowContext(ctx, q, table).Scan(&n); err == nil {
		return n > 0
	}
	q = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`
	if err := db.QueryRowContext(ctx, q, table).Scan(&n); err == nil {
		return n > 0
	}
	// fallback: 试 SELECT 1
	row, err := db.QueryContext(ctx, fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", table))
	if err != nil {
		return false
	}
	row.Close()
	return true
}

func (m *Migrator) getColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT 0", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cts, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	cols := make([]string, 0, len(cts))
	for _, c := range cts {
		cols = append(cols, c.Name())
	}
	return cols, nil
}

func (m *Migrator) buildUpsertSQL(table string, cols []string, dialect Dialect) string {
	placeholders := strings.Repeat("?,", len(cols))
	placeholders = placeholders[:len(placeholders)-1]
	colList := strings.Join(cols, ",")
	switch dialect {
	case DialectMySQL:
		// INSERT ... ON DUPLICATE KEY UPDATE id=id 让主键冲突静默, 等价"存在则忽略"
		return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE id=id", table, colList, placeholders)
	default: // SQLite
		return fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", table, colList, placeholders)
	}
}

func quoteJoined(cols []string, dialect Dialect) string {
	if dialect == DialectMySQL {
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = "`" + strings.ReplaceAll(c, "`", "``") + "`"
		}
		return strings.Join(quoted, ",")
	}
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}
	return strings.Join(quoted, ",")
}

// convertRowValues 修正 Scan 出来的 []byte 转 string, 以及其他类型差异
func convertRowValues(in []interface{}, colTypes []*sql.ColumnType, dialect Dialect) []interface{} {
	out := make([]interface{}, len(in))
	for i, v := range in {
		switch x := v.(type) {
		case []byte:
			out[i] = string(x)
		case bool:
			if dialect == DialectMySQL {
				if x {
					out[i] = int64(1)
				} else {
					out[i] = int64(0)
				}
			} else {
				if x {
					out[i] = int64(1)
				} else {
					out[i] = int64(0)
				}
			}
		default:
			out[i] = x
		}
		_ = colTypes
	}
	return out
}

func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "1062") ||
		strings.Contains(msg, "constraint failed")
}
