package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 1. 连接SQLite数据库
	dbPath := "data/dbops.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✓ Database connection successful (SQLite:", dbPath, ")")

	// 2. 检查users表并创建测试用户
	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&userCount)
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}

	if userCount == 0 {
		// 创建admin用户
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		_, err = db.Exec(`
			INSERT INTO users (username, password_hash, email, role, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, "admin", string(hashedPassword), "admin@example.com", "admin", "active")
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		fmt.Println("✓ Created admin user (username: admin, password: admin123)")
	} else {
		fmt.Println("✓ Admin user already exists")
	}

	// 3. 查询主机列表
	rows, err := db.Query("SELECT id, name, address, ssh_port, agent_port, os_type, status FROM hosts ORDER BY created_at DESC LIMIT 10")
	if err != nil {
		log.Fatalf("Failed to query hosts: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n=== Existing Hosts ===")
	hostCount := 0
	for rows.Next() {
		var id, name, address, osType, status string
		var sshPort, agentPort int
		if err := rows.Scan(&id, &name, &address, &sshPort, &agentPort, &osType, &status); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		hostCount++
		fmt.Printf("%d. [%s] %s (%s) - SSH:%d Agent:%d OS:%s Status:%s\n",
			hostCount, id, name, address, sshPort, agentPort, osType, status)
	}

	if hostCount == 0 {
		fmt.Println("No hosts found. You can add a host through the web interface or this tool.")
	}

	// 4. 测试说明
	fmt.Println("\n=== Next Steps ===")
	fmt.Println("1. Backend is running on http://localhost:8080")
	fmt.Println("2. Login with: username=admin, password=admin123")
	fmt.Println("3. Add a new Linux host to test environment check:")
	fmt.Println("   - Web interface: http://localhost:3000")
	fmt.Println("   - API endpoint: POST http://localhost:8080/api/v1/hosts")
	fmt.Println("\n4. Test Agent environment check API directly:")
	fmt.Println("   curl http://localhost:9090/agent/tasks/check-environment \\")
	fmt.Println("     -H 'Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16' \\")
	fmt.Println("     -H 'Content-Type: application/json' \\")
	fmt.Println("     -d '{\"check_tools\":true,\"check_resources\":true,\"check_network\":true}'")
}
