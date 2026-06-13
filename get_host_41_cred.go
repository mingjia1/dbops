package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "platform-backend/data/dbops.db")
	defer db.Close()

	var address, sshUser, sshCred string
	err := db.QueryRowContext(context.Background(), `
		SELECT address, ssh_user, ssh_credential
		FROM hosts WHERE address = '10.1.81.41'
	`).Scan(&address, &sshUser, &sshCred)

	if err != nil {
		fmt.Printf("41主机未在数据库中: %v\n", err)
		return
	}

	fmt.Printf("Address: %s\n", address)
	fmt.Printf("SSH User: %s\n", sshUser)
	fmt.Printf("SSH Credential (encrypted): %s...\n", sshCred[:min(20, len(sshCred))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
