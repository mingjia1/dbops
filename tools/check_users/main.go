package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "../../platform-backend/data/dbops.db")
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, username, password, role, status FROM users")
	if err != nil {
		log.Fatal("Failed to query users:", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, username, password, role, status string
		if err := rows.Scan(&id, &username, &password, &role, &status); err != nil {
			log.Fatal("Failed to scan row:", err)
		}
		fmt.Printf("ID: %s\nUsername: %s\nPassword: %s\nRole: %s\nStatus: %s\n\n", id, username, password, role, status)
	}
	if err := rows.Err(); err != nil {
		log.Fatal("Rows error:", err)
	}
	fmt.Println("--- end of users ---")
}
