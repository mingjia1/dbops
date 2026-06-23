package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

// RoleSwitchHistoryRepository P1: 持久化角色切换历史, 之前 SwitchService 走 in-memory map,
// 后端重启就清空, ListRoleSwitchHistory 返空. 现在落 role_switch_history 表.
type RoleSwitchHistoryRepository struct {
	db *Database
}

func NewRoleSwitchHistoryRepository(db *Database) *RoleSwitchHistoryRepository {
	return &RoleSwitchHistoryRepository{db: db}
}

func (r *RoleSwitchHistoryRepository) Create(ctx context.Context, rec *models.RoleSwitchRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO role_switch_history
		(id, cluster_id, cluster_type, instance_id, instance_host, old_role, new_role,
		 old_master_id, new_master_id, rebuilt_replicas, status, message, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.ClusterID, rec.ClusterType, rec.InstanceID, rec.InstanceHost,
		rec.OldRole, rec.NewRole, rec.OldMasterID, rec.NewMasterID,
		joinStrings(rec.RebuiltReplicas), rec.Status, rec.Message,
		nullableTime(&rec.StartedAt), nullableTime(&rec.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to insert role switch history: %w", err)
	}
	return nil
}

func (r *RoleSwitchHistoryRepository) ListByCluster(ctx context.Context, clusterID string, limit, offset int) ([]models.RoleSwitchRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, cluster_id, cluster_type, instance_id, instance_host, old_role, new_role,
			old_master_id, new_master_id, rebuilt_replicas, status, message, started_at, completed_at
		FROM role_switch_history
		WHERE cluster_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`,
		clusterID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list role switch history: %w", err)
	}
	defer rows.Close()

	out := make([]models.RoleSwitchRecord, 0)
	for rows.Next() {
		var rec models.RoleSwitchRecord
		var rebuilt string
		var startedAt, completedAt sql.NullTime
		var instanceHost sql.NullString
		if err := rows.Scan(&rec.ID, &rec.ClusterID, &rec.ClusterType, &rec.InstanceID, &instanceHost,
			&rec.OldRole, &rec.NewRole, &rec.OldMasterID, &rec.NewMasterID, &rebuilt,
			&rec.Status, &rec.Message, &startedAt, &completedAt); err != nil {
			return nil, err
		}
		rec.InstanceHost = instanceHost.String
		rec.RebuiltReplicas = splitStrings(rebuilt)
		if startedAt.Valid {
			rec.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			rec.CompletedAt = completedAt.Time
		}
		out = append(out, rec)
	}
	return out, nil
}

// RoleSwitchRecordStartTime 暴露给 SwitchService 在启动时记录时间 (避免服务重启后丢失).
func RoleSwitchRecordStartTime() time.Time {
	return time.Now().UTC()
}

func joinStrings(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}

func splitStrings(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
