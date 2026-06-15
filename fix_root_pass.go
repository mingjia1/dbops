package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	host := "10.1.81.16"
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	// Step 1: Stop MGR gracefully
	cmds := []string{
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "STOP GROUP_REPLICATION;" 2>&1`,
		`mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SET GLOBAL group_replication_start_on_boot=OFF;" 2>&1`,
	}
	for _, cmd := range cmds {
		s, _ := client.NewSession()
		out, _ := s.CombinedOutput(cmd)
		fmt.Printf("$ %s\n%s\n", cmd[:50], strings.TrimSpace(string(out)))
		s.Close()
	}

	// Step 2: Stop mysqld, start with skip-grant-tables
	cmds2 := []string{
		`mysqladmin -h127.0.0.1 -P3306 -urepl -prepl_123 shutdown 2>&1`,
		`sleep 2`,
		`nohup mysqld --skip-grant-tables --skip-networking --datadir=/data/mysql/3306 --socket=/tmp/mysql_skip.sock --pid-file=/tmp/mysql_skip.pid --user=mysql > /dev/null 2>&1 & sleep 2; echo started`,
	}
	for _, cmd := range cmds2 {
		s, _ := client.NewSession()
		out, _ := s.CombinedOutput(cmd)
		fmt.Printf("$ %s\n%s\n", cmd[:50], strings.TrimSpace(string(out)))
		s.Close()
	}

	// Step 3: Reset root password
	cmds3 := []string{
		`mysql --socket=/tmp/mysql_skip.sock -e "SELECT user, host, plugin FROM mysql.user WHERE user='root';" 2>&1`,
		`mysql --socket=/tmp/mysql_skip.sock -e "FLUSH PRIVILEGES; ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''; CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY ''; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES;" 2>&1`,
		`mysql --socket=/tmp/mysql_skip.sock -e "SELECT user, host, plugin FROM mysql.user WHERE user='root';" 2>&1`,
	}
	for _, cmd := range cmds3 {
		s, _ := client.NewSession()
		out, _ := s.CombinedOutput(cmd)
		fmt.Printf("$ %s\n%s\n", cmd[:50], strings.TrimSpace(string(out)))
		s.Close()
	}

	// Step 4: Stop skip-grant, start normal
	cmds4 := []string{
		`mysqladmin --socket=/tmp/mysql_skip.sock -uroot shutdown 2>&1`,
		`sleep 2`,
		`nohup mysqld --user=mysql --datadir=/data/mysql/3306 --port=3306 --bind-address=0.0.0.0 --socket=/data/mysql/3306/mysql.sock --pid-file=/data/mysql/3306/mysqld.pid --log-error=/data/mysql/3306/error.log --group_replication_start_on_boot=OFF > /dev/null 2>&1 & sleep 3; echo started`,
	}
	for _, cmd := range cmds4 {
		s, _ := client.NewSession()
		out, _ := s.CombinedOutput(cmd)
		fmt.Printf("$ %s\n%s\n", cmd[:50], strings.TrimSpace(string(out)))
		s.Close()
	}

	// Step 5: Test connection
	s, _ := client.NewSession()
	out, _ := s.CombinedOutput(`mysql -h127.0.0.1 -P3306 -uroot -e "SELECT 'root_ok' AS status;" 2>&1`)
	fmt.Printf("Test: %s\n", strings.TrimSpace(string(out)))
	s.Close()

	fmt.Println("Done - password reset to empty on", host)
}
