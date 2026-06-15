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

	host := "10.1.81.17"
	newPass := "Hcfc@DboOps#2024_80"
	grName := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("=== Fix %s ===\n", host)

	// Kill ALL mysqld
	fmt.Println("1. Kill MySQL")
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 3`)
	runCmd(client, `rm -f /data/mysql/3306/mysql_skip.sock /data/mysql/3306/mysqld_skip.pid`)

	// Start with skip-grant-tables
	fmt.Println("2. Start skip-grant")
	runCmd(client, `nohup /usr/sbin/mysqld --skip-grant-tables --skip-networking --datadir=/data/mysql/3306 --socket=/data/mysql/3306/mysql_skip.sock --pid-file=/data/mysql/3306/mysqld_skip.pid --user=mysql > /dev/null 2>&1 &`)
	if !waitSocket(client, "/data/mysql/3306/mysql_skip.sock", 5) {
		fmt.Println("   FAIL: socket not ready")
		return
	}
	fmt.Println("   socket ready")

	// Use UPDATE instead of ALTER USER (ALTER USER doesn't work in skip-grant-tables mode in MySQL 8.0)
	fmt.Println("3. Reset root password via UPDATE")
	// Generate proper authentication_string for mysql_native_password empty string
	// For mysql_native_password with empty string, the hash is empty
	sql := fmt.Sprintf("UPDATE mysql.user SET authentication_string='', plugin='mysql_native_password' WHERE user='root'; FLUSH PRIVILEGES;")
	out := runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql_skip.sock -e "%s" 2>&1`, strings.ReplaceAll(sql, "\"", "\\\"")))
	fmt.Printf("   UPDATE: %s\n", out)

	// Set password with ALTER USER (now that we flushed and root has no password via socket)
	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql_skip.sock -uroot --skip-password -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; CREATE USER IF NOT EXISTS 'root'@'%%%%' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%%%' WITH GRANT OPTION;" 2>&1`, newPass, newPass))
	fmt.Printf("   ALTER USER: %s\n", out)

	// Shutdown skip-grant
	fmt.Println("4. Shutdown skip-grant")
	runCmd(client, `mysqladmin --socket=/data/mysql/3306/mysql_skip.sock -uroot --skip-password shutdown 2>&1`)
	time.Sleep(3 * time.Second)
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 2`)

	// Start normal
	fmt.Println("5. Start MySQL normal")
	runCmd(client, `nohup /usr/sbin/mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log > /dev/null 2>&1 &`)
	if !waitSocket(client, "/data/mysql/3306/mysql.sock", 8) {
		fmt.Println("   FAIL: socket not ready")
		return
	}
	fmt.Println("   socket ready")
	time.Sleep(2 * time.Second)

	// Test password via TCP
	fmt.Println("6. Test password")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT 'TCP OK' AS status;" 2>&1`, newPass))
	fmt.Printf("   %s\n", out)

	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql.sock -uroot -p'%s' -e "SELECT 'SOCK OK' AS status;" 2>&1`, newPass))
	fmt.Printf("   %s\n", out)

	// GTID
	fmt.Println("7. GTID enable")
	for _, g := range []string{
		"SET GLOBAL ENFORCE_GTID_CONSISTENCY = ON",
		"SET GLOBAL GTID_MODE = OFF_PERMISSIVE",
		"SET GLOBAL GTID_MODE = ON_PERMISSIVE",
		"SET GLOBAL GTID_MODE = ON",
	} {
		runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, newPass, g))
	}

	// MGR config
	fmt.Println("8. Configure MGR")
	for _, c := range []string{
		fmt.Sprintf("SET GLOBAL group_replication_group_name = '%s'", grName),
		"SET GLOBAL group_replication_local_address = '10.1.81.17:33062'",
		"SET GLOBAL group_replication_group_seeds = '10.1.81.16:33061,10.1.81.17:33062,10.1.81.18:33063'",
		"SET GLOBAL group_replication_ip_allowlist = '10.1.81.16,10.1.81.17,10.1.81.18'",
		"SET GLOBAL group_replication_single_primary_mode = ON",
		"SET GLOBAL group_replication_enforce_update_everywhere_checks = OFF",
		"SET GLOBAL group_replication_start_on_boot = ON",
		"SET GLOBAL group_replication_recovery_get_public_key = ON",
		"CHANGE MASTER TO MASTER_USER='repl', MASTER_PASSWORD='repl_123' FOR CHANNEL 'group_replication_recovery'",
	} {
		runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "%s" 2>&1`, newPass, c))
	}

	// Start GR
	fmt.Println("9. Start Group Replication")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "START GROUP_REPLICATION;" 2>&1`, newPass))
	fmt.Printf("   %s\n", out)
	time.Sleep(3 * time.Second)

	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, newPass))
	fmt.Printf("   MGR: %s\n", out)

	fmt.Println("\n=== Done ===")
}
