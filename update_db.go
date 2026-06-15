package main

import (
	"context"
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

	// Update 17号端口
	result, err := db.ExecContext(context.Background(),
		"UPDATE instances SET port = 3307 WHERE host_id = (SELECT id FROM hosts WHERE address = '10.1.81.17')")
	if err != nil {
		log.Printf("Update 17 failed: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		fmt.Printf("Updated 17号 instance port to 3307 (%d rows)\n", rows)
	}

	// Update 18号端口
	result, err = db.ExecContext(context.Background(),
		"UPDATE instances SET port = 3308 WHERE host_id = (SELECT id FROM hosts WHERE address = '10.1.81.18')")
	if err != nil {
		log.Printf("Update 18 failed: %v", err)
	} else {
		rows, _ := result.RowsAffected()
		fmt.Printf("Updated 18号 instance port to 3308 (%d rows)\n", rows)
	}

	// Insert active cluster deployment
	_, err = db.ExecContext(context.Background(), `
		INSERT OR REPLACE INTO cluster_deployments (
			id, name, cluster_type, status, created_at, updated_at,
			primary_instance_id, config
		) VALUES (
			'mgr-manual-3node',
			'MGR 3-Node Production Cluster',
			'mgr',
			'completed',
			datetime('now'),
			datetime('now'),
			(SELECT id FROM instances WHERE host_id = (SELECT id FROM hosts WHERE address = '10.1.81.16') LIMIT 1),
			'{"nodes": 3, "version": "5.7.44", "deployment_method": "manual"}'
		)
	`)
	if err != nil {
		log.Printf("Insert cluster failed: %v", err)
	} else {
		fmt.Println("Created active MGR cluster deployment record")
	}

	fmt.Println("\nDatabase updated successfully")
}
