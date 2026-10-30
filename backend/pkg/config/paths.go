package config

import (
	"os"
	"path/filepath"
	"strings"
)

// P0-A / P1-6: 数据目录、元数据库、备份目录统一解析为绝对路径.
//
// 背景: 此前 viper 默认 data_dir="../db", main.go 又硬编码回退 "../db/dbops.db",
// 而 .env.example / config.yaml.example 写的是 "./data" —— 文档与代码三方各说各话;
// 且所有路径都相对进程 cwd, systemd/容器/手工启动的 WorkingDirectory 稍有不同,
// 数据与备份就会写到不同位置甚至静默新建目录.
//
// 解析规则 (优先级从高到低):
//  1. 显式配置 (config.yaml / DBOPS_DATA_DIR / DBOPS_SQLITE_PATH / DBOPS_BACKUP_DIR),
//     相对路径按启动时 cwd 解析为绝对路径;
//  2. data_dir 未配置时向后兼容: 旧部署的 ../db/dbops.db 存在则继续沿用 (由调用方
//     打弃用告警), 避免升级后静默切到空目录造成"数据丢失"的假象;
//  3. 否则启用 ./data, 与 .env.example / config.yaml.example 对齐.
//
// sqlite_path 为空时固定为 <data_dir>/dbops.db; backup_dir 为空时固定为
// <data_dir>/backups. 返回值一律为绝对路径.
const (
	legacyDataDir  = "../db"
	defaultDataDir = "data"
)

// resolveDataPaths 解析 data_dir 与 sqlite_path.
// usedLegacy=true 表示未显式配置 data_dir 且沿用了旧的 ../db 目录.
func resolveDataPaths(dataDir, sqlitePath string) (string, string, bool) {
	usedLegacy := false
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		if fileExists(filepath.Join(legacyDataDir, "dbops.db")) {
			dataDir = legacyDataDir
			usedLegacy = true
		} else {
			dataDir = defaultDataDir
		}
	}
	absDataDir := absPath(dataDir)

	sqlitePath = strings.TrimSpace(sqlitePath)
	if sqlitePath == "" {
		sqlitePath = filepath.Join(absDataDir, "dbops.db")
	} else {
		sqlitePath = absPath(sqlitePath)
	}
	return absDataDir, sqlitePath, usedLegacy
}

// resolveBackupDir 解析备份目录: 显式配置优先, 否则 <data_dir>/backups.
func resolveBackupDir(backupDir, absDataDir string) string {
	backupDir = strings.TrimSpace(backupDir)
	if backupDir == "" {
		backupDir = filepath.Join(absDataDir, "backups")
	}
	return absPath(backupDir)
}

// absPath 把相对路径按当前工作目录解析为绝对路径; 失败时原样返回 (不阻断启动).
func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
