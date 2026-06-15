package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func runCmd(client *ssh.Client, cmd string) string {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Sprintf("SESSION ERR: %v", err)
	}
	defer s.Close()
	out, _ := s.CombinedOutput(cmd)
	return strings.TrimSpace(string(out))
}

func waitSocket(client *ssh.Client, path string, sec int) bool {
	for i := 0; i < sec; i++ {
		if !strings.Contains(runCmd(client, fmt.Sprintf("ls %s 2>/dev/null || echo no", path)), "no") {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	host := "10.1.81.16"
	newPass := "Hcfc@DboOps#2024_80"

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("=== Fix root password on %s ===\n", host)

	// Kill MySQL
	fmt.Println("1. Kill MySQL")
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 3`)
	runCmd(client, `rm -f /data/mysql/3306/mysql_skip.sock /data/mysql/3306/mysqld_skip.pid`)

	// Start skip-grant
	fmt.Println("2. Start skip-grant")
	runCmd(client, `nohup /usr/sbin/mysqld --skip-grant-tables --skip-networking --datadir=/data/mysql/3306 --socket=/data/mysql/3306/mysql_skip.sock --pid-file=/data/mysql/3306/mysqld_skip.pid --user=mysql > /dev/null 2>&1 &`)
	if !waitSocket(client, "/data/mysql/3306/mysql_skip.sock", 8) {
		fmt.Println("FAIL: socket not ready")
		return
	}

	// UPDATE mysql.user
	fmt.Println("3. UPDATE mysql.user")
	sql := fmt.Sprintf("UPDATE mysql.user SET authentication_string='', plugin='mysql_native_password' WHERE user='root';")
	out := runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql_skip.sock -e "%s" 2>&1`, strings.ReplaceAll(sql, "\"", "\\\"")))
	fmt.Println("  UPDATE:", out)

	// FLUSH PRIVILEGES
	out = runCmd(client, `mysql --socket=/data/mysql/3306/mysql_skip.sock -e "FLUSH PRIVILEGES;" 2>&1`)
	fmt.Println("  FLUSH:", out)

	// Set password with ALTER (now root has no password, socket connection should work)
	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql_skip.sock -uroot -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; CREATE USER IF NOT EXISTS 'root'@'%%%%' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%%%' WITH GRANT OPTION;" 2>&1`, newPass, newPass))
	fmt.Println("  ALTER:", out)

	// Shutdown skip-grant
	fmt.Println("4. Shutdown skip-grant")
	runCmd(client, `mysqladmin --socket=/data/mysql/3306/mysql_skip.sock -uroot shutdown 2>&1`)
	time.Sleep(3 * time.Second)
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 2`)

	// Start normal with MGR off
	fmt.Println("5. Start MySQL normal (GR off)")
	runCmd(client, `nohup /usr/sbin/mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log --group_replication_start_on_boot=OFF > /dev/null 2>&1 &`)
	if !waitSocket(client, "/data/mysql/3306/mysql.sock", 8) {
		fmt.Println("FAIL: socket not ready")
		return
	}
	time.Sleep(2 * time.Second)

	// Test password
	fmt.Println("6. Test password")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT 'TCP OK' AS status;" 2>&1`, newPass))
	fmt.Println("  TCP:", out)

	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql.sock -uroot -p'%s' -e "SELECT 'SOCK OK' AS status;" 2>&1`, newPass))
	fmt.Println("  SOCK:", out)

	fmt.Println("\n=== Done on", host, "===")
}
