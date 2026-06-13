package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "d:/test_tmple/new_dbops/dbops/platform-backend/data/dbops.db")
	if err != nil {
		fmt.Printf("Failed to open DB: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Println("=== Recent MGR Deployments ===")
	rows, _ := db.Query(`
		SELECT id, status, progress, message, created_at
		FROM cluster_deployments
		WHERE id LIKE '%mgr%'
		ORDER BY created_at DESC
		LIMIT 10
	`)
	defer rows.Close()

	for rows.Next() {
		var id, status, message, createdAt string
		var progress int
		rows.Scan(&id, &status, &progress, &message, &createdAt)
		fmt.Printf("\nID: %s\nStatus: %s (%d%%)\nMessage: %s\nCreated: %s\n",
			id, status, progress, message, createdAt)
	}

	// 删除mgr-prod-16
	fmt.Println("\n=== Deleting mgr-prod-16 ===")
	result, _ := db.Exec("DELETE FROM cluster_deployments WHERE id = 'mgr-prod-16'")
	affected, _ := result.RowsAffected()
	fmt.Printf("Deleted %d rows\n", affected)

	// 验证删除
	fmt.Println("\n=== Verification ===")
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cluster_deployments WHERE id = 'mgr-prod-16'").Scan(&count)
	fmt.Printf("mgr-prod-16 count: %d\n", count)
}
