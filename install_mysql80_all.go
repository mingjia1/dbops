package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var hosts = []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"}

const (
	sshUser = "root"
	sshPass = "hcfc!2017"
)

func sshExec(ip, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
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

func printResult(ip, label, output string, err error) {
	if err != nil {
		fmt.Printf("[%s] %s ERROR: %v\n", ip, label, err)
	} else {
		fmt.Printf("[%s] %s OK: %s\n", ip, label, strings.TrimSpace(output))
	}
}

func main() {
	fmt.Println("=== Step 1: Stop MySQL and remove PXC 5.7 ===")
	for _, ip := range hosts {
		cmds := []string{
			"pkill -9 mysqld 2>/dev/null; sleep 2",
			"dpkg -l | grep -i percona | awk '{print $2}' | xargs -r dpkg --purge --force-depends 2>/dev/null",
			"apt-get remove -y percona-xtradb-cluster-server-5.7 percona-xtradb-cluster-client-5.7 percona-server-common-5.7 2>/dev/null",
			"apt-get autoremove -y 2>/dev/null",
			"rm -rf /data/mysql/3306/* /data/mysql/3306/.* 2>/dev/null",
			"rm -rf /var/log/mysql/* 2>/dev/null",
			"rm -f /etc/mysql/my.cnf /etc/my.cnf 2>/dev/null",
			"rm -rf /etc/mysql/percona* /etc/percona* 2>/dev/null",
		}
		for _, c := range cmds {
			out, err := sshExec(ip, c)
			label := c
		if len(label) > 50 {
			label = label[:50]
		}
		printResult(ip, label, out, err)
		}
		fmt.Printf("[%s] PXC cleaned\n", ip)
	}

	fmt.Println("\n=== Step 2: Install mysql-server-8.0 ===")
	// Pre-seed root password for non-interactive install
	for _, ip := range hosts {
		out, err := sshExec(ip, `DEBIAN_FRONTEND=noninteractive apt-get install -y mysql-server-8.0 2>&1`)
		printResult(ip, "apt install mysql-server-8.0", out, err)
	}

	fmt.Println("\n=== Step 3: Stop default mysql, set up platform datadir ===")
	for _, ip := range hosts {
		cmds := []string{
			"systemctl stop mysql 2>/dev/null; sleep 2",
			"pkill -9 mysqld 2>/dev/null; sleep 1",
			"rm -rf /data/mysql/3306/* /data/mysql/3306/.* 2>/dev/null",
			"mkdir -p /data/mysql/3306",
			"chown -R mysql:mysql /data/mysql/3306",
		}
		for _, c := range cmds {
			sshExec(ip, c)
		}

		// Initialize with --initialize-insecure
		initCmd := "mysqld --initialize-insecure --user=mysql --datadir=/data/mysql/3306 2>&1"
		out, err := sshExec(ip, initCmd)
		printResult(ip, "mysqld --initialize-insecure", out, err)

		// Start with correct port and datadir
		startCmd := "nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log --skip-grant-tables > /dev/null 2>&1 & sleep 3; echo STARTED"
		out, err = sshExec(ip, startCmd)
		printResult(ip, "mysqld start with skip-grant-tables", out, err)

		// Verify running
		out, _ = sshExec(ip, "mysqladmin ping -h127.0.0.1 -P3306 -uroot 2>&1")
		printResult(ip, "mysql ping", out, nil)
	}

	fmt.Println("\n=== Step 4: Configure MySQL 8.0 for MGR ===")
	for _, ip := range hosts {
		// Create root user with password auth (since skip-grant-tables is active)
		cmds := []string{
			"FLUSH PRIVILEGES",
			"ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''",
			"CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY ''",
			"GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION",
			"GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION",
			"FLUSH PRIVILEGES",
		}
		for _, c := range cmds {
			escaped := strings.ReplaceAll(c, "'", "'\\''")
			out, _ := sshExec(ip, fmt.Sprintf("mysql -h127.0.0.1 -P3306 -uroot -e '%s' 2>&1", escaped))
			if !strings.Contains(out, "ERROR") && !strings.Contains(out, "Warning") {
				fmt.Printf("[%s] %s OK\n", ip, c[:60])
			} else if strings.Contains(out, "already exists") || strings.Contains(out, "0 rows affected") || strings.Contains(out, "1 row affected") {
				fmt.Printf("[%s] %s OK\n", ip, c[:60])
			} else {
				fmt.Printf("[%s] %s => %s\n", ip, c[:60], strings.TrimSpace(out))
			}
		}
	}

	fmt.Println("\n=== Step 5: Restart MySQL normally (without skip-grant-tables) ===")
	for _, ip := range hosts {
		sshExec(ip, "pkill -9 mysqld 2>/dev/null; sleep 2")
		startCmd := "nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log > /dev/null 2>&1 & sleep 3; echo STARTED"
		out, err := sshExec(ip, startCmd)
		printResult(ip, "restart normal", out, err)

		out, _ = sshExec(ip, "mysqladmin ping -h127.0.0.1 -P3306 -uroot 2>&1")
		printResult(ip, "verify running", out, nil)

		out, _ = sshExec(ip, "mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT VERSION()' 2>&1")
		printResult(ip, "version", out, nil)
	}

	fmt.Println("\n=== Step 6: Enable group_replication plugin and verify ===")
	for _, ip := range hosts {
		out, _ := sshExec(ip, "mysql -h127.0.0.1 -P3306 -uroot -e 'INSTALL PLUGIN group_replication SONAME \"group_replication.so\"; SELECT PLUGIN_NAME, PLUGIN_STATUS FROM information_schema.PLUGINS WHERE PLUGIN_NAME = \"group_replication\"' 2>&1")
		fmt.Printf("[%s] group_replication: %s\n", ip, strings.TrimSpace(out))
	}

	fmt.Println("\n=== DONE ===")
}
