package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var hosts = []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"}

func sshExec(ip, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	client, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return "", err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

func runMySQL(ip, socket, sql string) string {
	// Use heredoc to avoid quoting issues
	cmd := fmt.Sprintf("mysql -S %s -uroot -e \"%s\" 2>&1", socket, strings.ReplaceAll(sql, "\"", "\\\""))
	out, _ := sshExec(ip, cmd)
	return out
}

func main() {
	for _, ip := range hosts {
		fmt.Printf("\n========== %s ==========\n", ip)

		// Kill any existing mysqld
		sshExec(ip, "pkill -9 mysqld 2>/dev/null; sleep 2")

		// Check if MySQL 8.0 is installed
		out, _ := sshExec(ip, "which mysqld && mysqld --version 2>&1 || echo 'NOT_INSTALLED'")
		fmt.Printf("  MySQL: %s\n", strings.TrimSpace(out))

		if strings.Contains(out, "NOT_INSTALLED") {
			fmt.Printf("  MySQL 8.0 not installed, skipping\n")
			continue
		}

		// Make sure directories exist
		sshExec(ip, "mkdir -p /var/lib/mysql-files /var/lib/mysql-keyring /var/run/mysqld /data/mysql/3306")
		sshExec(ip, "chown -R mysql:mysql /var/lib/mysql-files /var/lib/mysql-keyring /var/run/mysqld /data/mysql")

		// Initialize if needed
		out, _ = sshExec(ip, "ls /data/mysql/3306/auto.cnf 2>/dev/null || echo 'NEED_INIT'")
		if strings.Contains(out, "NEED_INIT") {
			sshExec(ip, "rm -rf /data/mysql/3306/* /data/mysql/3306/.* 2>/dev/null")
			sshExec(ip, "mysqld --initialize-insecure --user=mysql --datadir=/data/mysql/3306 2>&1")
			fmt.Printf("  Initialized\n")
		} else {
			fmt.Printf("  Already initialized\n")
		}

		// Start MySQL normally
		startCmd := "nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log > /dev/null 2>&1 & sleep 3; echo STARTED"
		out, _ = sshExec(ip, startCmd)
		fmt.Printf("  Start: %s\n", strings.TrimSpace(out))

		socket := "/data/mysql/3306/mysql.sock"

		// Test socket connection
		out = runMySQL(ip, socket, "SELECT 1 AS ok")
		fmt.Printf("  Socket test: %s\n", strings.TrimSpace(out))

		// Configure root for TCP access
		sql := "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''"
		out = runMySQL(ip, socket, sql)
		fmt.Printf("  Auth config: %s\n", strings.TrimSpace(out))

		sql = "CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY ''"
		out = runMySQL(ip, socket, sql)
		fmt.Printf("  Root user: %s\n", strings.TrimSpace(out))

		sql = "GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION"
		out = runMySQL(ip, socket, sql)
		fmt.Printf("  Root grant: %s\n", strings.TrimSpace(out))

		sql = "FLUSH PRIVILEGES"
		out = runMySQL(ip, socket, sql)
		fmt.Printf("  Flush: %s\n", strings.TrimSpace(out))

		// Test TCP connection
		out, _ = sshExec(ip, "mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT VERSION()' 2>&1")
		fmt.Printf("  TCP test: %s\n", strings.TrimSpace(out))

		// Install group_replication plugin
		out = runMySQL(ip, socket, "INSTALL PLUGIN group_replication SONAME 'group_replication.so'")
		fmt.Printf("  Install GR: %s\n", strings.TrimSpace(out))

		out = runMySQL(ip, socket, "SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME='group_replication'")
		fmt.Printf("  GR plugin: %s\n", strings.TrimSpace(out))
	}
}
