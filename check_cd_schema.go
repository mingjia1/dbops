package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "d:/test_tmple/new_dbops/dbops/platform-backend/data/dbops.db")
	defer db.Close()

	fmt.Println("=== cluster_deployments schema ===")
	rows, err := db.Query("PRAGMA table_info(cluster_deployments)")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		fmt.Printf("  %s (%s)\n", name, ctype)
	}
	rows.Close()

	var count int
	db.QueryRow("SELECT COUNT(*) FROM cluster_deployments").Scan(&count)
	fmt.Printf("\nTotal rows: %d\n", count)

	// 查看所有部署记录
	fmt.Println("\n=== All deployments ===")
	rows2, _ := db.Query("SELECT id, status FROM cluster_deployments ORDER BY id")
	for rows2.Next() {
		var id, status string
		rows2.Scan(&id, &status)
		fmt.Printf("  %s: %s\n", id, status)
	}
	rows2.Close()
}
