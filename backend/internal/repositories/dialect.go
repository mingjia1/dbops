package repositories

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DialectCompat 把"两种方言的差异"集中到这里, 让 repository 写一份 SQL 即可.
// 主要差异点:
//   1. 占位符: MySQL 用 ?, SQLite 用 ?
//      (其实一样, 都是 ?, 所以这里不需要改; 但保留扩展能力)
//   2. 自增主键: MySQL AUTO_INCREMENT, SQLite AUTOINCREMENT (但我们用 UUID 主键, 不依赖自增)
//   3. ON UPDATE CURRENT_TIMESTAMP: MySQL 支持, SQLite 需要 trigger (我们用应用层 UpdatedAt)
//   4. BOOLEAN: MySQL 是 TINYINT(1), SQLite 原生支持, 需 Scan 到 bool
//   5. DECIMAL: 两端语法一致
//   6. TEXT: 两端一致
//   7. ENGINE=InnoDB / CHARSET=utf8mb4: MySQL 专属, SQLite 不支持

// BoolColumn 返回当前方言下 BOOLEAN 列的类型定义
func (d Dialect) BoolColumn() string {
	if d == DialectMySQL {
		return "TINYINT(1) DEFAULT 0"
	}
	return "INTEGER DEFAULT 0"
}

// TimestampColumn 返回 TIMESTAMP DEFAULT CURRENT_TIMESTAMP 子句
func (d Dialect) TimestampColumn() string {
	if d == DialectMySQL {
		return "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
	}
	return "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
}

// NullTimestampColumn 返回可空 TIMESTAMP (无默认值)
func (d Dialect) NullTimestampColumn() string {
	if d == DialectMySQL {
		return "TIMESTAMP NULL"
	}
	return "TIMESTAMP"
}

// DateColumn 返回 DATE 列定义
func (d Dialect) DateColumn() string {
	return "DATE"
}

// EngineClause 仅 MySQL 末尾需要 ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
func (d Dialect) EngineClause() string {
	if d == DialectMySQL {
		return " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
	}
	return ""
}

// ForeignKeyClause SQLite 需要 PRAGMA foreign_keys=ON (已经在 DSN 中启用),
// CREATE TABLE 内 FOREIGN KEY 语法两端一致.
func (d Dialect) ForeignKeyClause() string { return "" }

// IndexClause 两端都支持 CREATE INDEX / 表内 INDEX
func (d Dialect) IndexClause() string { return "" }

// NowFunc 返回当前时间, 两端都用 time.Now(), 但要保持 UTC 便于跨时区
func NowFunc() time.Time {
	return time.Now().UTC()
}

// FormatTime 数据库存储用 UTC, 显示时前端做时区转换
func FormatTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

// ScanBool 把数据库里的 TINYINT(1)/INTEGER 0/1 解到 *bool
func ScanBool(v sql.NullInt64) bool {
	return v.Valid && v.Int64 != 0
}

// ValueBool 把 bool 写回数据库
func ValueBool(b bool) interface{} {
	if b {
		return int64(1)
	}
	return int64(0)
}

// QuoteIdent 引用列名 (例如 `condition` 是 MySQL 关键字)
func (d Dialect) QuoteIdent(name string) string {
	if d == DialectMySQL {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// SQLBoolEqual 生成 boolean 列比较表达式 (例如 WHERE ssl_enabled = ?)
func (d Dialect) SQLBoolEqual(b bool) interface{} {
	if b {
		return int64(1)
	}
	return int64(0)
}

// EnsureDirectory 确保目录存在
func EnsureDirectory(dir string) error {
	if dir == "" {
		return nil
	}
	return mkdirAll(dir)
}

// BuildInsertSQL 构造 INSERT INTO ... VALUES (?,?,?) (兼容两端)
func BuildInsertSQL(table string, columns []string) string {
	placeholders := strings.Repeat("?,", len(columns))
	placeholders = placeholders[:len(placeholders)-1]
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(columns, ","), placeholders)
}
