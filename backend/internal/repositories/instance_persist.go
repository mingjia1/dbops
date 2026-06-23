package repositories

import "github.com/jackcode/mysql-ops-platform/internal/models"

// instancePersist 与 instance_repository 共享的 JSON 持久化结构.
// 放在这里避免 instance_repository 仍然引用旧的内嵌结构 (SQLite 化后已不再用).
type instancePersist struct {
	Mems  []*models.Instance                  `json:"mems"`
	Conns map[string]models.InstanceConnection `json:"conns"`
	Vers  map[string]models.InstanceVersion     `json:"vers"`
}
