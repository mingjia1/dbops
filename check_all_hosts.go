package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "data/dbops.db")
	defer db.Close()

	fmt.Println("=== 所有主机列表 ===")
	rows, _ := db.Query("SELECT id, name, address, ssh_user, agent_port FROM hosts ORDER BY address")
	for rows.Next() {
		var id, name, address, sshUser string
		var agentPort int
		rows.Scan(&id, &name, &address, &sshUser, &agentPort)
		fmt.Printf("%s | %-15s | user:%-6s | agent:%d\n", address, name, sshUser, agentPort)
	}
	rows.Close()

	fmt.Println("\n=== 检查是否有41主机 ===")
	var count int
	db.QueryRow("SELECT COUNT(*) FROM hosts WHERE address LIKE '%41%'").Scan(&count)
	fmt.Printf("包含'41'的主机: %d 台\n", count)
}
