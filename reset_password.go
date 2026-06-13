package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbPath := "data/dbops.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// 查看现有用户
	rows, err := db.Query("SELECT id, username, email, role, status, created_at FROM users LIMIT 5")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== Existing Users ===")
	for rows.Next() {
		var id, username, email, role, status, createdAt string
		if err := rows.Scan(&id, &username, &email, &role, &status, &createdAt); err != nil {
			log.Printf("Scan failed: %v", err)
			continue
		}
		fmt.Printf("ID: %s\n  Username: %s\n  Email: %s\n  Role: %s\n  Status: %s\n  Created: %s\n\n",
			id, username, email, role, status, createdAt)
	}

	// 更新admin密码为admin123
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Hash failed: %v", err)
	}

	// 使用第一个用户
	result, err := db.Exec("UPDATE users SET password = ? WHERE id = (SELECT id FROM users ORDER BY created_at LIMIT 1)", string(hashedPassword))
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	affected, _ := result.RowsAffected()
	fmt.Printf("✓ Updated %d user(s) password to: admin123\n", affected)
	fmt.Println("✓ Use username: codexadmin131813, password: admin123")
}
