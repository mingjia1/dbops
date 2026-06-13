package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "d:/test_tmple/new_dbops/dbops/platform-backend/data/dbops.db")
	defer db.Close()

	fmt.Println("=== Hosts in database ===")
	rows, _ := db.Query(`
		SELECT id, name, address, port, status, agent_port
		FROM hosts
		ORDER BY created_at DESC
	`)
	defer rows.Close()

	for rows.Next() {
		var id, name, address, status string
		var port, agentPort int
		rows.Scan(&id, &name, &address, &port, &status, &agentPort)
		fmt.Printf("\nID: %s\nName: %s\nAddress: %s\nPort: %d\nAgent Port: %d\nStatus: %s\n",
			id, name, address, port, agentPort, status)
	}

	fmt.Println("\n=== Host with ID 61168e37-9022-43e2-8722-7100218ad8b3 ===")
	var name, address string
	var port, agentPort int
	db.QueryRow(`
		SELECT name, address, port, agent_port
		FROM hosts
		WHERE id = '61168e37-9022-43e2-8722-7100218ad8b3'
	`).Scan(&name, &address, &port, &agentPort)
	fmt.Printf("Name: %s\nAddress: %s\nPort: %d\nAgent Port: %d\n",
		name, address, port, agentPort)

	fmt.Printf("\nBackend would call: http://%s:%d/agent/tasks/deploy\n", address, agentPort)
}
