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
		Timeout:         60 * time.Second,
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

func main() {
	for _, ip := range hosts {
		fmt.Printf("\n========== Processing %s ==========\n", ip)

		// Kill MySQL
		sshExec(ip, "pkill -9 mysqld 2>/dev/null; pkill -9 mysql 2>/dev/null; sleep 2")

		// Force remove ALL percona packages
		cmds := []string{
			"dpkg --purge --force-depends percona-xtradb-cluster-server-5.7 percona-xtradb-cluster-client-5.7 percona-xtradb-cluster-common-5.7 2>&1",
			"dpkg --purge --force-depends percona-server-common percona-server-server percona-server-source percona-xtrabackup-24 2>&1",
			"dpkg --purge --force-depends percona-release percona-telemetry-agent 2>&1",
			"dpkg --purge --force-depends percona-xtrabackup-80 2>&1",
			// Remove residual configs
			"rm -rf /etc/mysql /etc/my.cnf /etc/percona* /var/lib/mysql* /var/log/mysql 2>/dev/null",
			"rm -rf /data/mysql/3306/* /data/mysql/3306/.* 2>/dev/null",
			"mkdir -p /data/mysql/3306",
		}
		for _, c := range cmds {
			out, _ := sshExec(ip, c)
			if out != "" {
				firstLine := strings.Split(out, "\n")[0]
				fmt.Printf("  %s\n", firstLine[:min(len(firstLine), 100)])
			}
		}

		// Verify PXC is gone
		out, _ := sshExec(ip, "dpkg -l 2>/dev/null | grep -i percona || echo 'NO PERCONA'")
		fmt.Printf("  Percona packages: %s\n", strings.TrimSpace(out))

		// Now install mysql-server-8.0 with noninteractive frontend
		fmt.Printf("  Installing mysql-server-8.0...\n")
		out, err := sshExec(ip, "DEBIAN_FRONTEND=noninteractive apt-get install -y mysql-server-8.0 2>&1 | tail -5")
		if err != nil {
			fmt.Printf("  Install ERROR: %v\n", err)
			fmt.Printf("  %s\n", out)
			continue
		}
		fmt.Printf("  Install done: %s\n", strings.TrimSpace(out))

		// Check the version after install
		out, _ = sshExec(ip, "mysqld --version 2>&1")
		fmt.Printf("  Version: %s\n", strings.TrimSpace(out))

		// Stop the default mysql service
		sshExec(ip, "systemctl stop mysql 2>/dev/null; sleep 2; pkill -9 mysqld 2>/dev/null; sleep 1")

		// Initialize with platform datadir
		out, _ = sshExec(ip, "mysqld --initialize-insecure --user=mysql --datadir=/data/mysql/3306 --log-error=/data/mysql/3306/init.log 2>&1")
		fmt.Printf("  Init: %s\n", strings.TrimSpace(out))

		// Start MySQL
		startCmd := "nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log --skip-grant-tables > /dev/null 2>&1 & sleep 3; echo STARTED"
		out, _ = sshExec(ip, startCmd)
		fmt.Printf("  Start: %s\n", strings.TrimSpace(out))

		// Verify
		out, _ = sshExec(ip, "mysqladmin ping -h127.0.0.1 -P3306 -uroot 2>&1")
		fmt.Printf("  Ping: %s\n", strings.TrimSpace(out))

		if strings.Contains(out, "alive") {
			// Configure root
			cmds := []string{
				"mysql -h127.0.0.1 -P3306 -uroot -e \"FLUSH PRIVILEGES;\" 2>&1",
				"mysql -h127.0.0.1 -P3306 -uroot -e \"ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '';\" 2>&1",
				"mysql -h127.0.0.1 -P3306 -uroot -e \"CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY ''; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;\" 2>&1",
			}
			for _, c := range cmds {
				out, _ := sshExec(ip, c)
				trim := strings.TrimSpace(out)
				if trim != "" {
					fmt.Printf("  Config: %s\n", trim)
				}
			}

			// Stop and restart normally
			sshExec(ip, "pkill -9 mysqld 2>/dev/null; sleep 2")
			out, _ = sshExec(ip, "nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log > /dev/null 2>&1 & sleep 3; mysqladmin ping -h127.0.0.1 -P3306 -uroot 2>&1")
			fmt.Printf("  Final ping: %s\n", strings.TrimSpace(out))

			// Test connection
			out, _ = sshExec(ip, "mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT VERSION()' 2>&1")
			fmt.Printf("  Final test: %s\n", strings.TrimSpace(out))
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
