package repositories

import (
	"context"
	"fmt"
)

// PluginRecord 对应既有 plugin_registry 表 (M1 迁移已建表, 此前无调用方).
// Config 存放 HTTPPluginConfig 的 JSON, 重启时据此恢复动态插件.
type PluginRecord struct {
	Name    string
	Type    string
	Version string
	Config  string
}

type PluginRepository struct {
	db *Database
}

func NewPluginRepository(db *Database) *PluginRepository {
	return &PluginRepository{db: db}
}

func (r *PluginRepository) List(ctx context.Context) ([]PluginRecord, error) {
	if r == nil || r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("plugin repository not initialized")
	}
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT name, plugin_type, version, COALESCE(config, '') FROM plugin_registry WHERE status = 'active' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PluginRecord
	for rows.Next() {
		var rec PluginRecord
		if err := rows.Scan(&rec.Name, &rec.Type, &rec.Version, &rec.Config); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Upsert: DELETE + INSERT 兼容 MySQL 与 SQLite, 免维护两套 ON CONFLICT 语法.
// name 同时作主键 id (表上 name 有唯一索引), 无需额外 UUID 依赖.
func (r *PluginRepository) Upsert(ctx context.Context, rec PluginRecord) error {
	if r == nil || r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("plugin repository not initialized")
	}
	if _, err := r.db.Pool.ExecContext(ctx, `DELETE FROM plugin_registry WHERE name = ?`, rec.Name); err != nil {
		return err
	}
	_, err := r.db.Pool.ExecContext(ctx,
		`INSERT INTO plugin_registry (id, name, plugin_type, version, status, config) VALUES (?, ?, ?, ?, 'active', ?)`,
		rec.Name, rec.Name, rec.Type, rec.Version, rec.Config)
	return err
}

func (r *PluginRepository) Delete(ctx context.Context, name string) error {
	if r == nil || r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("plugin repository not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM plugin_registry WHERE name = ?`, name)
	return err
}
