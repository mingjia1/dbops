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

	// 检查集群信息
	fmt.Println("=== 集群信息 ===")
	var id, name, clusterType, status string
	var createdAt string
	err = db.QueryRow(`
		SELECT id, name, cluster_type, status, created_at
		FROM cluster_deployments
		WHERE name = ?
	`, clusterName).Scan(&id, &name, &clusterType, &status, &createdAt)

	if err == sql.ErrNoRows {
		fmt.Printf("❌ 集群 %s 不存在\n", clusterName)
		fmt.Println("\n当前所有集群:")
		rows, _ := db.Query(`SELECT name, cluster_type, status FROM cluster_deployments LIMIT 10`)
		defer rows.Close()
		for rows.Next() {
			var n, ct, st string
			rows.Scan(&n, &ct, &st)
			fmt.Printf("  - %s (%s) [%s]\n", n, ct, st)
		}
		return
	} else if err != nil {
		log.Fatal("查询集群失败:", err)
	}

	fmt.Printf("集群ID: %s\n", id)
	fmt.Printf("集群名称: %s\n", name)
	fmt.Printf("集群类型: %s\n", clusterType)
	fmt.Printf("当前状态: %s\n", status)
	fmt.Printf("创建时间: %s\n\n", createdAt)

	// 检查关联实例
	fmt.Println("=== 关联实例 ===")
	instRows, err := db.Query(`
		SELECT i.id, i.name, i.cluster_id, ic.host, ic.port
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
		var instID, instName, clusterID, host sql.NullString
		var port sql.NullInt64
		instRows.Scan(&instID, &instName, &clusterID, &host, &port)
		instanceCount++
		fmt.Printf("实例 %d:\n", instanceCount)
		fmt.Printf("  ID: %s\n", instID.String)
		fmt.Printf("  名称: %s\n", instName.String)
		if host.Valid && port.Valid {
			fmt.Printf("  地址: %s:%d\n", host.String, port.Int64)
		}
	}

	if instanceCount == 0 {
		fmt.Println("✓ 该集群没有关联实例")
	} else {
		fmt.Printf("\n共 %d 个关联实例\n", instanceCount)
	}

	// 检查拓扑信息
	fmt.Println("\n=== 拓扑信息 ===")
	topoRows, err := db.Query(`
		SELECT it.instance_id, it.cluster_id, it.master_id, it.slave_ids, it.replication_mode
		FROM instance_topologies it
		INNER JOIN instances i ON it.instance_id = i.id
		WHERE i.cluster_id = ?
	`, id)
	if err == nil {
		defer topoRows.Close()
		topoCount := 0
		for topoRows.Next() {
			var instID, clustID, masterID, slaveIDs, replMode sql.NullString
			topoRows.Scan(&instID, &clustID, &masterID, &slaveIDs, &replMode)
			topoCount++
			fmt.Printf("实例 %s:\n", instID.String)
			fmt.Printf("  ClusterID: %s\n", clustID.String)
			fmt.Printf("  MasterID: %s\n", masterID.String)
			fmt.Printf("  SlaveIDs: %s\n", slaveIDs.String)
			fmt.Printf("  ReplMode: %s\n", replMode.String)
		}
		if topoCount == 0 {
			fmt.Println("✓ 无拓扑信息")
		}
	}

	// 提示
	fmt.Println("\n=== 操作建议 ===")
	if status == "destroyed" {
		fmt.Println("✓ 集群已经处于 destroyed 状态")
	} else if instanceCount == 0 {
		fmt.Println("→ 集群无实例，可以直接删除元数据")
	} else {
		fmt.Println("→ 集群有实例，需要执行完整销毁流程（备份→验证→销毁）")
		fmt.Println("→ 使用 API: DELETE /api/deployments/" + id)
	}
}
