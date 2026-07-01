package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type RoleRepository struct {
	db *Database
}

func NewRoleRepository(db *Database) *RoleRepository {
	return &RoleRepository{db: db}
}

var BuiltinRoles = []models.Role{
	{Name: "admin", DisplayName: "管理员", Description: "平台管理员", Permissions: []string{"*"}, IsBuiltin: true},
	{Name: "dba", DisplayName: "DBA", Description: "数据库运维", Permissions: []string{"instance:*", "deploy:*", "upgrade:*", "backup:*", "restore:*", "monitor:view", "host:read", "audit:view"}, IsBuiltin: true},
	{Name: "operator", DisplayName: "操作员", Description: "日常操作", Permissions: []string{"instance:view", "deploy:execute", "backup:execute", "restore:execute", "monitor:view", "host:read"}, IsBuiltin: true},
	{Name: "developer", DisplayName: "开发者", Description: "研发只读与申请", Permissions: []string{"instance:view_own", "backup:apply", "monitor:view_own"}, IsBuiltin: true},
	{Name: "auditor", DisplayName: "审计员", Description: "审计只读", Permissions: []string{"instance:view", "monitor:view", "audit:view"}, IsBuiltin: true},
	{Name: "viewer", DisplayName: "只读", Description: "平台只读", Permissions: []string{"instance:view", "host:read", "monitor:view"}, IsBuiltin: true},
}

func (r *RoleRepository) SeedBuiltinRoles(ctx context.Context) error {
	for _, role := range BuiltinRoles {
		if existing, err := r.GetByName(ctx, role.Name); err != nil {
			return err
		} else if existing != nil {
			continue
		}
		if err := r.Create(ctx, &role); err != nil {
			return err
		}
	}
	return nil
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	now := time.Now()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}
	role.UpdatedAt = now
	perms, err := json.Marshal(role.Permissions)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.ExecContext(ctx, `
		INSERT INTO roles (id, name, display_name, description, permissions, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		role.ID, role.Name, role.DisplayName, role.Description, string(perms), boolInt(role.IsBuiltin), role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	perms, err := json.Marshal(role.Permissions)
	if err != nil {
		return err
	}
	role.UpdatedAt = time.Now()
	res, err := r.db.Pool.ExecContext(ctx, `
		UPDATE roles SET display_name = ?, description = ?, permissions = ?, updated_at = ?
		WHERE id = ? AND is_builtin = 0`,
		role.DisplayName, role.Description, string(perms), role.UpdatedAt, role.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("role not found or builtin role cannot be updated")
	}
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `DELETE FROM roles WHERE id = ? AND is_builtin = 0`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("role not found or builtin role cannot be deleted")
	}
	return nil
}

func (r *RoleRepository) List(ctx context.Context) ([]models.Role, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, name, display_name, description, permissions, is_builtin, created_at, updated_at
		FROM roles ORDER BY is_builtin DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []models.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	return roles, rows.Err()
}

func (r *RoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	row := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, display_name, description, permissions, is_builtin, created_at, updated_at
		FROM roles WHERE name = ?`, name)
	role, err := scanRole(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return role, err
}

func (r *RoleRepository) ListByUserID(ctx context.Context, userID string) ([]models.Role, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT r.id, r.name, r.display_name, r.description, r.permissions, r.is_builtin, r.created_at, r.updated_at
		FROM roles r INNER JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = ?
		ORDER BY r.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []models.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	return roles, rows.Err()
}

func (r *RoleRepository) SetUserRolesByName(ctx context.Context, userID string, roleNames []string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	tx, err := r.db.Pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, name := range roleNames {
		var roleID string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, name).Scan(&roleID); err != nil {
			return fmt.Errorf("role %s not found: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)`, userID, roleID, time.Now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanRole(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.Role, error) {
	var role models.Role
	var permissions string
	var builtin int
	if err := scanner.Scan(&role.ID, &role.Name, &role.DisplayName, &role.Description, &permissions, &builtin, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(permissions), &role.Permissions)
	role.IsBuiltin = builtin != 0
	return &role, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
