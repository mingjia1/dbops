package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

// HostRepository 直接走 SQL (MySQL 或 SQLite), 不再有 in-memory 分支.
// 因为 SQLite 已经替我们做了"零配置持久化" - 启动时 Database 已自动创建并跑了 migrations.
type HostRepository struct {
	db *Database
}

func NewHostRepository(db *Database) *HostRepository {
	return &HostRepository{db: db}
}

// AttachStore 保留兼容旧调用. 现在 SQLite 本身就是持久化层, 不再需要 JSON 兜底.
// 如果 db 是 nil (未初始化), 才退到 JSON 存储.
func (r *HostRepository) AttachStore(store *JSONStore) {
	// 当前架构: db 始终可用 (Database.NewDatabase 内置 SQLite 回退), 无需 JSON 兜底.
	// 保留空方法体仅供 main.go 调用.
	_ = store
}

func (r *HostRepository) Create(ctx context.Context, host *models.Host) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	if host.ID == "" {
		host.ID = uuid.New().String()
	}
	if host.Status == "" {
		host.Status = "unknown"
	}
	now := time.Now().UTC()
	host.CreatedAt = now
	host.UpdatedAt = now

	// 检查重名
	var existing string
	err := r.db.Pool.QueryRowContext(ctx, `SELECT id FROM hosts WHERE name = ? LIMIT 1`, host.Name).Scan(&existing)
	if err == nil {
		return fmt.Errorf("host with name %s already exists", host.Name)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check host name: %w", err)
	}

	_, err = r.db.Pool.ExecContext(ctx, `
		INSERT INTO hosts (id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential,
			agent_port, os_type, description, tags, status, last_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		host.ID, host.Name, host.Address, host.SSHPort, host.SSHUser, host.SSHAuthMethod,
		nullableString(host.SSHCredential), host.AgentPort, host.OSType, host.Description, host.Tags, host.Status,
		nullableTime(host.LastCheckAt), host.CreatedAt, host.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	return nil
}

func (r *HostRepository) GetByID(ctx context.Context, id string) (*models.Host, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential, agent_port,
			os_type, description, tags, status, last_check_at, created_at, updated_at
		FROM hosts WHERE id = ?`, id)
	host := &models.Host{}
	var lastCheckAt sql.NullTime
	var sshCredential sql.NullString
	if err := row.Scan(
		&host.ID, &host.Name, &host.Address, &host.SSHPort, &host.SSHUser, &host.SSHAuthMethod,
		&sshCredential, &host.AgentPort, &host.OSType, &host.Description, &host.Tags, &host.Status,
		&lastCheckAt, &host.CreatedAt, &host.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("host not found")
		}
		return nil, fmt.Errorf("failed to get host: %w", err)
	}
	if lastCheckAt.Valid {
		t := lastCheckAt.Time
		host.LastCheckAt = &t
	}
	if sshCredential.Valid {
		host.SSHCredential = sshCredential.String
	}
	return host, nil
}

func (r *HostRepository) List(ctx context.Context, limit, offset int) ([]models.Host, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, name, address, ssh_port, ssh_user, ssh_auth_method, ssh_credential, agent_port,
			os_type, description, tags, status, last_check_at, created_at, updated_at
		FROM hosts ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}
	defer rows.Close()

	hosts := make([]models.Host, 0)
	for rows.Next() {
		var h models.Host
		var lastCheckAt sql.NullTime
		var sshCredential sql.NullString
		if err := rows.Scan(
			&h.ID, &h.Name, &h.Address, &h.SSHPort, &h.SSHUser, &h.SSHAuthMethod,
			&sshCredential, &h.AgentPort, &h.OSType, &h.Description, &h.Tags, &h.Status,
			&lastCheckAt, &h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastCheckAt.Valid {
			t := lastCheckAt.Time
			h.LastCheckAt = &t
		}
		if sshCredential.Valid {
			h.SSHCredential = sshCredential.String
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (r *HostRepository) Update(ctx context.Context, host *models.Host) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	host.UpdatedAt = time.Now().UTC()
	// P1-3: 之前 UPDATE 不检查 RowsAffected, host 已删除时 update 返 nil 假装成功.
	// 修: 拿 res.RowsAffected, 0 行返 not found, 业务层能感知.
	res, err := r.db.Pool.ExecContext(ctx, `
		UPDATE hosts SET name = ?, address = ?, ssh_port = ?, ssh_user = ?, ssh_auth_method = ?,
			ssh_credential = ?, agent_port = ?, os_type = ?, description = ?, tags = ?, updated_at = ?
		WHERE id = ?`,
		host.Name, host.Address, host.SSHPort, host.SSHUser, host.SSHAuthMethod,
		host.SSHCredential, host.AgentPort, host.OSType, host.Description, host.Tags, host.UpdatedAt, host.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update host: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("host not found: %s", host.ID)
	}
	return nil
}

func (r *HostRepository) UpdateStatus(ctx context.Context, id, status string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	now := time.Now().UTC()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE hosts SET status = ?, last_check_at = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update host status: %w", err)
	}
	return nil
}

func (r *HostRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}
	return nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC()
}
