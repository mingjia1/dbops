package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mysql" {
		runMySQL()
		return
	}
	runSQLite()
}

func runSQLite() {
	dbPath := "data/dbops.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("Database not found at %s", dbPath)
	}

	dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	resetPassword(db)
}

func runMySQL() {
	dsn := os.Getenv("DBOPS_DB_URL")
	if dsn == "" {
		log.Fatal("DBOPS_DB_URL is required for mysql mode")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping MySQL: %v", err)
	}
	fmt.Println("Connected to MySQL successfully")

	resetPassword(db)
}

func resetPassword(db *sql.DB) {
	// List existing users
	rows, err := db.Query("SELECT id, username, role, email, status FROM users ORDER BY username")
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}
	fmt.Println("=== Existing Users ===")
	hasAdmin := false
	type User struct{ id, username, role, email, status string }
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.id, &u.username, &u.role, &u.email, &u.status); err != nil {
			log.Fatalf("Failed to scan: %v", err)
		}
		users = append(users, u)
		fmt.Printf("  id=%s user=%s role=%s email=%s status=%s\n", u.id, u.username, u.role, u.email, u.status)
		if u.username == "admin" {
			hasAdmin = true
		}
	}
	rows.Close()

	password := adminPasswordFromInput()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	if hasAdmin {
		// Update admin user's password
		result, err := db.Exec("UPDATE users SET password = ? WHERE username = ?", string(hash), "admin")
		if err != nil {
			log.Fatalf("Failed to update admin password: %v", err)
		}
		affected, _ := result.RowsAffected()
		fmt.Printf("\n=== Admin Password Updated ===\n")
		fmt.Printf("Rows affected: %d\n", affected)
	} else {
		// Create admin user
		fmt.Println("\n=== Creating admin user ===")
		result, err := db.Exec("INSERT INTO users (id, username, password, email, role, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			"admin-seed-001", "admin", string(hash), "admin@localhost", "admin", "active", time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		affected, _ := result.RowsAffected()
		fmt.Printf("Admin user created. Rows affected: %d\n", affected)
	}
	fmt.Println("Password updated. Plaintext password was not logged.")
	fmt.Println("======================")
}

func adminPasswordFromInput() string {
	if len(os.Args) > 2 {
		return os.Args[2]
	}
	if password := os.Getenv("DBOPS_ADMIN_PASSWORD"); password != "" {
		return password
	}
	log.Fatal("new admin password is required as arg 2 or DBOPS_ADMIN_PASSWORD")
	return ""
}
