package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	// 检查旧库
	fmt.Println("=== 检查 dbops.db（旧库）===")
	db1, _ := sql.Open("sqlite", "data/dbops.db")
	defer db1.Close()

	var count1 int
	db1.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&count1)
	fmt.Printf("主机数量: %d\n", count1)

	rows1, _ := db1.Query("SELECT id, name, address FROM hosts LIMIT 5")
	fmt.Println("\n前5条主机记录:")
	for rows1.Next() {
		var id, name, address string
		rows1.Scan(&id, &name, &address)
		fmt.Printf("  %s | %s | %s\n", id, name, address)
	}
	rows1.Close()

	// 检查新库
	fmt.Println("\n\n=== 检查 dbops_platform.db（新库）===")
	db2, _ := sql.Open("sqlite", "data/dbops_platform.db")
	defer db2.Close()

	var count2 int
	err := db2.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&count2)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
	} else {
		fmt.Printf("主机数量: %d\n", count2)
	}
}
