package config

import (
	"os"
	"path/filepath"
	"testing"
)

// P0-A: 显式配置的相对路径必须解析为绝对路径, 空值走默认规则.
func TestResolveDataPaths_ExplicitRelative(t *testing.T) {
	dataDir, sqlitePath, usedLegacy := resolveDataPaths("./data", "")
	if usedLegacy {
		t.Errorf("usedLegacy = true, want false for explicit data_dir")
	}
	if !filepath.IsAbs(dataDir) {
		t.Errorf("dataDir = %q, want absolute path", dataDir)
	}
	if !filepath.IsAbs(sqlitePath) {
		t.Errorf("sqlitePath = %q, want absolute path", sqlitePath)
	}
	if filepath.Base(sqlitePath) != "dbops.db" {
		t.Errorf("sqlitePath = %q, want base dbops.db", sqlitePath)
	}
	if filepath.Dir(sqlitePath) != dataDir {
		t.Errorf("sqlitePath dir = %q, want dataDir %q", filepath.Dir(sqlitePath), dataDir)
	}
}

func TestResolveDataPaths_ExplicitSQLitePath(t *testing.T) {
	_, sqlitePath, _ := resolveDataPaths("./data", "./custom/meta.db")
	if !filepath.IsAbs(sqlitePath) {
		t.Errorf("sqlitePath = %q, want absolute path", sqlitePath)
	}
	if filepath.Base(sqlitePath) != "meta.db" {
		t.Errorf("sqlitePath = %q, want base meta.db", sqlitePath)
	}
}

// 未配置 data_dir 且旧目录 ../db/dbops.db 存在 -> 兼容沿用 (usedLegacy=true).
func TestResolveDataPaths_LegacyFallback(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(root, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "db", "dbops.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	dataDir, sqlitePath, usedLegacy := resolveDataPaths("", "")
	if !usedLegacy {
		t.Errorf("usedLegacy = false, want true when legacy ../db/dbops.db exists")
	}
	if filepath.Base(dataDir) != "db" || !usedLegacy {
		t.Errorf("dataDir = %q (usedLegacy=%v), want legacy ../db", dataDir, usedLegacy)
	}
	if !filepath.IsAbs(dataDir) || !filepath.IsAbs(sqlitePath) {
		t.Errorf("dataDir=%q sqlitePath=%q, want absolute", dataDir, sqlitePath)
	}
}

// 未配置 data_dir 且无旧目录 -> 落到 ./data, 与 .env.example 对齐.
func TestResolveDataPaths_DefaultToData(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "app")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	dataDir, sqlitePath, usedLegacy := resolveDataPaths("", "")
	if usedLegacy {
		t.Errorf("usedLegacy = true, want false when legacy dir absent")
	}
	if filepath.Base(dataDir) != "data" {
		t.Errorf("dataDir = %q, want .../data", dataDir)
	}
	if sqlitePath != filepath.Join(dataDir, "dbops.db") {
		t.Errorf("sqlitePath = %q, want %q", sqlitePath, filepath.Join(dataDir, "dbops.db"))
	}
}

func TestResolveBackupDir(t *testing.T) {
	dataDir := t.TempDir()
	got := resolveBackupDir("", dataDir)
	if got != filepath.Join(dataDir, "backups") {
		t.Errorf("default backupDir = %q, want %q", got, filepath.Join(dataDir, "backups"))
	}
	custom := resolveBackupDir("./nightly", dataDir)
	if !filepath.IsAbs(custom) {
		t.Errorf("custom backupDir = %q, want absolute path", custom)
	}
}
