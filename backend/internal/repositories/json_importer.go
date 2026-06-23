package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

// JSONImporter 把旧的 data/*.json 文件一次性导入 SQLite.
// 设计原则:
//   - 只在 SQLite 模式下触发 (MySQL 不需要)
//   - 仅当目标表为空时执行 (避免覆盖用户已经迁移到 MySQL 的数据)
//   - 导入成功后, 旧文件改名为 *.imported-YYYYMMDD, 不删除
//     方便用户回滚, 不会因为脏写导致数据丢失
type JSONImporter struct {
	dataDir string
}

func NewJSONImporter(dataDir string) *JSONImporter {
	return &JSONImporter{dataDir: dataDir}
}

// ImportAll 扫描 dataDir 下的 hosts.json / instances.json 并导入到 SQLite.
// 返回 (importedCount, error). 错误非致命, 打印 warning 即可.
func (im *JSONImporter) ImportAll(ctx context.Context, db *Database) (int, error) {
	if db == nil || !db.IsSQLite() {
		return 0, nil
	}
	if im.dataDir == "" {
		return 0, nil
	}
	if _, err := os.Stat(im.dataDir); os.IsNotExist(err) {
		return 0, nil
	}

	total := 0

	// 1. hosts
	if n, err := im.importHosts(ctx, db); err != nil {
		return total, fmt.Errorf("import hosts: %w", err)
	} else {
		total += n
	}

	// 2. instances (含 connections / versions)
	if n, err := im.importInstances(ctx, db); err != nil {
		return total, fmt.Errorf("import instances: %w", err)
	} else {
		total += n
	}

	return total, nil
}

func (im *JSONImporter) importHosts(ctx context.Context, db *Database) (int, error) {
	var count int
	var existing string
	if err := db.Pool.QueryRowContext(ctx, `SELECT id FROM hosts LIMIT 1`).Scan(&existing); err == nil {
		// 表里已有数据, 跳过
		return 0, nil
	}

	raw, err := os.ReadFile(filepath.Join(im.dataDir, "hosts.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var hosts []*models.Host
	if err := json.Unmarshal(raw, &hosts); err != nil {
		return 0, fmt.Errorf("parse hosts.json: %w", err)
	}
	if len(hosts) == 0 {
		return 0, nil
	}

	tx, err := db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO hosts (id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential,
			agent_port, os_type, description, tags, status, last_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, h := range hosts {
		created := h.CreatedAt
		if created.IsZero() {
			created = time.Now().UTC()
		}
		updated := h.UpdatedAt
		if updated.IsZero() {
			updated = created
		}
		_, err := stmt.ExecContext(ctx,
			h.ID, h.Name, h.Address, h.SSHPort, h.SSHUser, h.SSHAuthMethod,
			nullableString(h.SSHCredential), h.AgentPort, h.OSType, h.Description, h.Tags, h.Status,
			nullableTime(h.LastCheckAt), created, updated,
		)
		if err != nil {
			return count, fmt.Errorf("insert host %s: %w", h.ID, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	im.archiveOld("hosts.json")
	return count, nil
}

func (im *JSONImporter) importInstances(ctx context.Context, db *Database) (int, error) {
	var existing string
	if err := db.Pool.QueryRowContext(ctx, `SELECT id FROM instances LIMIT 1`).Scan(&existing); err == nil {
		return 0, nil
	}

	raw, err := os.ReadFile(filepath.Join(im.dataDir, "instances.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var p instancePersist
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0, fmt.Errorf("parse instances.json: %w", err)
	}
	if len(p.Mems) == 0 {
		return 0, nil
	}

	tx, err := db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	instStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO instances (id, name, cluster_id, host_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer instStmt.Close()

	connStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO instance_connections (id, instance_id, host, port, username, password_encrypted, ssl_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer connStmt.Close()

	verStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer verStmt.Close()

	count := 0
	for _, inst := range p.Mems {
		created := inst.CreatedAt
		if created.IsZero() {
			created = time.Now().UTC()
		}
		updated := inst.UpdatedAt
		if updated.IsZero() {
			updated = created
		}
		_, err := instStmt.ExecContext(ctx,
			inst.ID, inst.Name, nullableString(inst.ClusterID), nullableStringPtr(inst.HostID),
			created, updated,
		)
		if err != nil {
			return count, fmt.Errorf("insert instance %s: %w", inst.ID, err)
		}
		count++

		// 关联 connection
		if inst.Connection.InstanceID == "" {
			for _, c := range p.Conns {
				if c.InstanceID == inst.ID {
					_, _ = connStmt.ExecContext(ctx, c.ID, c.InstanceID, c.Host, c.Port, c.Username, c.PasswordEncrypted, c.SSLEnabled)
					break
				}
			}
		}
		// 关联 version
		if inst.Version.InstanceID == "" {
			for _, v := range p.Vers {
				if v.InstanceID == inst.ID {
					rd := v.ReleaseDate
					ed := v.EOLDate
					_, _ = verStmt.ExecContext(ctx, v.ID, v.InstanceID, v.Flavor, v.Version, v.FullVersion,
						nullableTime(&rd), nullableTime(&ed), v.IsLTS, v.Features, v.Engines)
					break
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	im.archiveOld("instances.json")
	return count, nil
}

func (im *JSONImporter) archiveOld(name string) {
	old := filepath.Join(im.dataDir, name)
	if _, err := os.Stat(old); os.IsNotExist(err) {
		return
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	newName := fmt.Sprintf("%s.imported-%s", name, stamp)
	_ = os.Rename(old, filepath.Join(im.dataDir, newName))
}
