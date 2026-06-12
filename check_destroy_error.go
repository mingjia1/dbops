package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./platform-backend/data/dbops.db")
	if err != nil {
		log.Fatal("打开数据库失败:", err)
	}
	defer db.Close()

	clusterName := "ha57-for-mha-20260610-v1"

	// 1. 检查集群信息
	fmt.Println("=== 1. 集群信息 ===")
	var id, name, clusterType, status string
	var createdAt string
	err = db.QueryRow(`
		SELECT id, name, cluster_type, status, created_at
		FROM cluster_deployments
		WHERE name = ? OR id = ?
	`, clusterName, clusterName).Scan(&id, &name, &clusterType, &status, &createdAt)

	if err == sql.ErrNoRows {
		fmt.Printf("❌ 未找到集群 %s\n", clusterName)
		return
	} else if err != nil {
		log.Fatal("查询集群失败:", err)
	}

	fmt.Printf("集群ID: %s\n", id)
	fmt.Printf("集群名称: %s\n", name)
	fmt.Printf("集群类型: %s\n", clusterType)
	fmt.Printf("当前状态: %s\n", status)
	fmt.Printf("创建时间: %s\n\n", createdAt)

	// 2. 检查审计日志
	fmt.Println("=== 2. 销毁操作审计日志 ===")
	rows, err := db.Query(`
		SELECT operation, result, error_msg, created_at
		FROM audit_logs
		WHERE resource_id = ? AND operation = 'destroy_cluster'
		ORDER BY created_at DESC
		LIMIT 5
	`, id)
	if err != nil {
		log.Fatal("查询审计日志失败:", err)
	}
	defer rows.Close()

	hasDestroyLog := false
	for rows.Next() {
		var op, result, errMsg, created string
		if err := rows.Scan(&op, &result, &errMsg, &created); err != nil {
			log.Fatal(err)
		}
		hasDestroyLog = true
		fmt.Printf("时间: %s\n", created)
		fmt.Printf("操作: %s\n", op)
		fmt.Printf("结果: %s\n", result)
		fmt.Printf("错误信息: %s\n\n", errMsg)
	}

	if !hasDestroyLog {
		fmt.Println("⚠️  未找到销毁操作的审计日志\n")
	}

	// 3. 检查关联实例
	fmt.Println("=== 3. 关联实例 ===")
	instRows, err := db.Query(`
		SELECT i.id, i.name, ic.host, ic.port
		FROM instances i
		LEFT JOIN instance_connections ic ON i.id = ic.instance_id
		WHERE i.cluster_id = ?
	`, id)
	if err != nil {
		log.Fatal("查询实例失败:", err)
	}
	defer instRows.Close()

	instanceCount := 0
	for instRows.Next() {
		var instID, instName, host sql.NullString
		var port sql.NullInt64
		if err := instRows.Scan(&instID, &instName, &host, &port); err != nil {
			log.Fatal(err)
		}
		instanceCount++
		fmt.Printf("实例 %d: %s (%s)\n", instanceCount, instName.String, instID.String)
		if host.Valid && port.Valid {
			fmt.Printf("  地址: %s:%d\n", host.String, port.Int64)
		}
	}

	if instanceCount == 0 {
		fmt.Println("✓ 该集群没有关联实例\n")
	} else {
		fmt.Printf("\n共 %d 个关联实例\n\n", instanceCount)
	}

	// 4. 检查备份记录
	fmt.Println("=== 4. 相关备份记录 ===")
	backupRows, err := db.Query(`
		SELECT b.id, b.instance_id, b.backup_type, b.status, b.file_path, b.file_size, b.created_at
		FROM backups b
		INNER JOIN instances i ON b.instance_id = i.id
		WHERE i.cluster_id = ?
		ORDER BY b.created_at DESC
		LIMIT 5
	`, id)
	if err != nil {
		log.Fatal("查询备份记录失败:", err)
	}
	defer backupRows.Close()

	hasBackup := false
	for backupRows.Next() {
		var backupID, instanceID, backupType, status, filePath string
		var fileSize int64
		var created string
		if err := backupRows.Scan(&backupID, &instanceID, &backupType, &status, &filePath, &fileSize, &created); err != nil {
			log.Fatal(err)
		}
		hasBackup = true
		fmt.Printf("备份ID: %s\n", backupID)
		fmt.Printf("实例ID: %s\n", instanceID)
		fmt.Printf("类型: %s\n", backupType)
		fmt.Printf("状态: %s\n", status)
		fmt.Printf("文件: %s\n", filePath)
		fmt.Printf("大小: %d bytes\n", fileSize)
		fmt.Printf("时间: %s\n\n", created)
	}

	if !hasBackup {
		fmt.Println("⚠️  未找到相关备份记录\n")
	}

	// 5. 诊断建议
	fmt.Println("=== 5. 诊断建议 ===")
	if !hasDestroyLog {
		fmt.Println("1. 检查后端日志文件查看详细错误")
		fmt.Println("2. 确认API请求是否真正到达DestroyCluster方法")
	} else if instanceCount > 0 && !hasBackup {
		fmt.Println("1. 集群有关联实例但无备份记录")
		fmt.Println("2. 可能是备份执行失败导致销毁中断")
		fmt.Println("3. 建议检查：")
		fmt.Println("   - Agent服务是否正常")
		fmt.Println("   - 实例连接信息是否正确")
		fmt.Println("   - 备份存储路径是否有权限")
	} else if hasBackup {
		fmt.Println("1. 已有备份记录，检查最近备份的状态")
		fmt.Println("2. 如果备份失败，需要修复备份问题")
		fmt.Println("3. 如果备份成功但验证失败，检查validateRemovalBackup逻辑")
	}
}
