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
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT operation, resource_id, result, error_msg, created_at
		FROM audit_logs
		WHERE resource_id LIKE '%ha57%' OR resource_id LIKE '%20260610-v1%'
		ORDER BY created_at DESC LIMIT 10
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var op, rid, result, errMsg, created string
		if err := rows.Scan(&op, &rid, &result, &errMsg, &created); err != nil {
			log.Fatal(err)
		}
		found = true
		fmt.Printf("操作: %s\n资源: %s\n结果: %s\n错误: %s\n时间: %s\n\n", op, rid, result, errMsg, created)
	}

	if !found {
		fmt.Println("❌ 未找到该集群的销毁审计记录")
		fmt.Println("\n可能原因：")
		fmt.Println("1. 后端API返回成功但实际未执行销毁逻辑")
		fmt.Println("2. 集群无关联实例，触发了'no instances found'错误")
		fmt.Println("3. 备份服务未配置，触发了'backup service is required'错误")
		fmt.Println("\n检查方法：查看后端日志或Web页面审计日志")
	}
}
