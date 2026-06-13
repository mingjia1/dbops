package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "d:/test_tmple/new_dbops/dbops/platform-backend/data/dbops.db")
	defer db.Close()

	hostID := "61168e37-9022-43e2-8722-7100218ad8b3"

	fmt.Printf("=== Query host %s ===\n", hostID)
	row := db.QueryRowContext(context.Background(), `
		SELECT id, name, address, ssh_port, agent_port, status
		FROM hosts WHERE id = ?`, hostID)

	var id, name, address, status string
	var sshPort, agentPort int

	if err := row.Scan(&id, &name, &address, &sshPort, &agentPort, &status); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("ID: %s\n", id)
	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Address: '%s'\n", address)
	fmt.Printf("SSH Port: %d\n", sshPort)
	fmt.Printf("Agent Port: %d\n", agentPort)
	fmt.Printf("Status: %s\n", status)

	if address == "" {
		fmt.Println("\n❌ ADDRESS IS EMPTY!")
	} else {
		fmt.Printf("\n✅ Address is: '%s'\n", address)
	}
}
