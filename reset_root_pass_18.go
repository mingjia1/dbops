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

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	host := "10.1.81.18"
	newPass := "Hcfc@DboOps#2024_80"

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("=== Reset root password on %s to %q ===\n", host, newPass)

	// Kill ALL MySQL processes
	fmt.Println("\n1. Kill all mysqld processes")
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 3`)
	fmt.Println("   done")

	// Remove stale socket/pid files
	fmt.Println("2. Clean up stale files")
	runCmd(client, `rm -f /data/mysql/3306/mysql_skip.sock /data/mysql/3306/mysqld_skip.pid /var/log/mysql_skip.log /tmp/mysql_skip.*`)
	fmt.Println("   done")

	// Start with skip-grant-tables (NO FLUSH PRIVILEGES will be used)
	fmt.Println("3. Start MySQL with --skip-grant-tables")
	runCmd(client, `nohup /usr/sbin/mysqld --skip-grant-tables --skip-networking --datadir=/data/mysql/3306 --socket=/data/mysql/3306/mysql_skip.sock --pid-file=/data/mysql/3306/mysqld_skip.pid --user=mysql > /var/log/mysql_skip.log 2>&1 &`)
	time.Sleep(3 * time.Second)
	out := runCmd(client, `ls /data/mysql/3306/mysql_skip.sock 2>/dev/null && echo "socket ready" || echo "socket NOT ready"`)
	fmt.Printf("   %s\n", out)

	// Check root users (skip-grant mode, no auth needed)
	fmt.Println("4. Current root users (skip-grant mode)")
	out = runCmd(client, `mysql --socket=/data/mysql/3306/mysql_skip.sock -e "SELECT user, host, plugin FROM mysql.user WHERE user='root';" 2>&1`)
	fmt.Printf("   %s\n", out)

	// ALTER USERS - IMPORTANT: do NOT FLUSH PRIVILEGES while in skip-grant mode!
	// FLUSH PRIVILEGES re-enables grant tables and then we'd need auth for ALTER USER
	fmt.Println("5. Set root password (without FLUSH PRIVILEGES)")
	sql := fmt.Sprintf("ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; CREATE USER IF NOT EXISTS 'root'@'%%%%' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%%%' WITH GRANT OPTION;", newPass, newPass)
	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql_skip.sock -e "%s" 2>&1`, strings.ReplaceAll(sql, "\"", "\\\"")))
	fmt.Printf("   %s\n", out)

	// Shutdown skip-grant MySQL (still no auth needed since we haven't flushed)
	fmt.Println("6. Shutdown skip-grant MySQL")
	out = runCmd(client, `mysqladmin --socket=/data/mysql/3306/mysql_skip.sock -uroot shutdown 2>&1`)
	fmt.Printf("   %s\n", out)
	time.Sleep(3 * time.Second)

	// Kill any remaining mysqld
	runCmd(client, `pkill -9 -f mysqld 2>/dev/null; sleep 2`)

	// Clean up skip files
	runCmd(client, `rm -f /data/mysql/3306/mysql_skip.sock /data/mysql/3306/mysqld_skip.pid`)

	// Start MySQL normally (no GR yet)
	fmt.Println("7. Start MySQL in normal mode")
	runCmd(client, `nohup /usr/sbin/mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log --group_replication_start_on_boot=OFF > /dev/null 2>&1 &`)
	time.Sleep(4 * time.Second)
	out = runCmd(client, `ls /data/mysql/3306/mysql.sock 2>/dev/null && echo "socket ready" || echo "socket NOT ready"`)
	fmt.Printf("   %s\n", out)

	// Test password via TCP
	fmt.Println("8. Test new password")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT 'TCP OK' AS status;" 2>&1`, newPass))
	fmt.Printf("   TCP: %s\n", out)

	// Test via socket
	out = runCmd(client, fmt.Sprintf(`mysql --socket=/data/mysql/3306/mysql.sock -uroot -p'%s' -e "SELECT 'SOCK OK' AS status;" 2>&1`, newPass))
	fmt.Printf("   Socket: %s\n", out)

	// Start MGR
	fmt.Println("9. Start Group Replication")
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SET GLOBAL group_replication_start_on_boot=ON; START GROUP_REPLICATION;" 2>&1`, newPass))
	fmt.Printf("   %s\n", out)
	time.Sleep(2 * time.Second)

	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members WHERE member_host='ops';" 2>&1`, newPass))
	fmt.Printf("   MGR: %s\n", out)

	fmt.Println("\n=== DONE ===")
}
