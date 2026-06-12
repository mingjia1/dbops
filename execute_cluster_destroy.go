package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 打开数据库
	db, err := sql.Open("sqlite3", "./platform-backend/data/dbops.db")
	if err != nil {
		log.Fatal("打开数据库失败:", err)
	}
	defer db.Close()

	ctx := context.Background()
	clusterName := "ha57-for-mha-20260610-v1"

	// 1. 查找集群
	var clusterID, status string
	err = db.QueryRowContext(ctx, `
		SELECT id, status FROM cluster_deployments WHERE name = ?
	`, clusterName).Scan(&clusterID, &status)

	if err == sql.ErrNoRows {
		log.Printf("集群 %s 不存在", clusterName)
		return
	} else if err != nil {
		log.Fatal("查询集群失败:", err)
	}

	fmt.Printf("找到集群: %s (状态: %s)\n", clusterID, status)

	if status == "destroyed" {
		fmt.Println("✓ 集群已经是 destroyed 状态，无需再次销毁")
		return
	}

	// 2. 查找关联实例
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, cluster_id FROM instances WHERE cluster_id = ?
	`, clusterID)
	if err != nil {
		log.Fatal("查询实例失败:", err)
	}

	var instances []struct {
		ID        string
		Name      string
		ClusterID string
	}

	for rows.Next() {
		var inst struct {
			ID        string
			Name      string
			ClusterID string
		}
		rows.Scan(&inst.ID, &inst.Name, &inst.ClusterID)
		instances = append(instances, inst)
	}
	rows.Close()

	fmt.Printf("找到 %d 个关联实例\n", len(instances))

	// 3. 开始事务执行销毁
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal("开始事务失败:", err)
	}
	defer tx.Rollback()

	// 4. 清除每个实例的拓扑信息
	for _, inst := range instances {
		fmt.Printf("  清理实例 %s (%s) 的拓扑信息...\n", inst.Name, inst.ID)

		// 清除拓扑
		_, err = tx.ExecContext(ctx, `
			UPDATE instance_topologies
			SET cluster_id = '', master_id = '', slave_ids = '', replication_mode = ''
			WHERE instance_id = ?
		`, inst.ID)
		if err != nil {
			log.Printf("    警告: 清除拓扑失败: %v", err)
		} else {
			fmt.Printf("    ✓ 拓扑信息已清除\n")
		}

		// 更新实例状态
		_, err = tx.ExecContext(ctx, `
			UPDATE instance_statuses
			SET run_status = 'stopped', health_status = 'offline',
			    role = '', replication_status = '', seconds_behind_master = -1
			WHERE instance_id = ?
		`, inst.ID)
		if err != nil {
			log.Printf("    警告: 更新状态失败: %v", err)
		} else {
			fmt.Printf("    ✓ 实例状态已更新为 stopped/offline\n")
		}

		// 清除集群关联
		_, err = tx.ExecContext(ctx, `
			UPDATE instances SET cluster_id = '' WHERE id = ?
		`, inst.ID)
		if err != nil {
			log.Fatal("清除实例集群关联失败:", err)
		}
		fmt.Printf("    ✓ 集群关联已清除\n")
	}

	// 5. 更新集群状态为 destroyed
	result, err := tx.ExecContext(ctx, `
		UPDATE cluster_deployments SET status = 'destroyed' WHERE id = ?
	`, clusterID)
	if err != nil {
		log.Fatal("更新集群状态失败:", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		log.Fatal("未找到要更新的集群")
	}

	// 6. 记录审计日志
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (id, user_id, operation, resource_type, resource_id, result, error_msg, created_at)
		VALUES (?, 'system', 'destroy_cluster', 'cluster', ?, 'success',
		        'Manual cluster destruction via cleanup script - topology and instances cleared', datetime('now'))
	`, generateID(), clusterID)
	if err != nil {
		log.Printf("警告: 创建审计日志失败: %v", err)
	}

	// 7. 提交事务
	if err := tx.Commit(); err != nil {
		log.Fatal("提交事务失败:", err)
	}

	fmt.Println("\n✅ 集群销毁完成")
	fmt.Printf("集群ID: %s\n", clusterID)
	fmt.Printf("集群名称: %s\n", clusterName)
	fmt.Printf("清理的实例数: %d\n", len(instances))
	fmt.Println("操作内容:")
	fmt.Println("  - 清除所有实例的 cluster_id")
	fmt.Println("  - 清除所有拓扑信息 (master_id, slave_ids, replication_mode)")
	fmt.Println("  - 更新实例状态为 stopped/offline")
	fmt.Println("  - 更新集群状态为 destroyed")
	fmt.Println("  - 记录审计日志")

	// 8. 验证结果
	fmt.Println("\n=== 验证结果 ===")

	// 验证集群状态
	var newStatus string
	db.QueryRowContext(ctx, `SELECT status FROM cluster_deployments WHERE id = ?`, clusterID).Scan(&newStatus)
	fmt.Printf("集群状态: %s\n", newStatus)

	// 验证实例关联
	var remainingCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM instances WHERE cluster_id = ?`, clusterID).Scan(&remainingCount)
	if remainingCount == 0 {
		fmt.Println("✓ 所有实例的集群关联已清除")
	} else {
		fmt.Printf("⚠ 仍有 %d 个实例关联到该集群\n", remainingCount)
	}

	// 验证拓扑信息
	var topoCount int
	db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM instance_topologies
		WHERE cluster_id = ? AND (cluster_id != '' OR master_id != '' OR slave_ids != '')
	`, clusterID).Scan(&topoCount)
	if topoCount == 0 {
		fmt.Println("✓ 所有拓扑信息已清除")
	} else {
		fmt.Printf("⚠ 仍有 %d 个实例有拓扑信息\n", topoCount)
	}
}

func generateID() string {
	return fmt.Sprintf("audit-%d", 1003141)
}
