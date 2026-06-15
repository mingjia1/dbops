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

	host := "10.1.81.17"

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("=== Check error log and MySQL state on %s ===\n", host)

	// Check log
	out := runCmd(client, `tail -30 /data/mysql/3306/error.log 2>/dev/null || echo "no log"`)
	fmt.Println("Error log tail:")
	fmt.Println(out)

	// Check running MySQL
	out = runCmd(client, `ps aux | grep mysqld | grep -v grep || echo "no mysqld"`)
	fmt.Println("\nMySQL processes:")
	fmt.Println(out)

	// Check ports
	out = runCmd(client, `ss -tlnp 2>/dev/null | grep 3306 || netstat -tlnp 2>/dev/null | grep 3306 || echo "no 3306 ports"`)
	fmt.Println("\nPorts:")
	fmt.Println(out)

	// Check socket
	out = runCmd(client, `ls -la /data/mysql/3306/mysql.sock 2>/dev/null || echo "no socket"`)
	fmt.Println("\nSocket:", out)

	// Test socket connection
	out = runCmd(client, `mysql --socket=/data/mysql/3306/mysql.sock -uroot -e "SELECT 'socket works' AS s;" 2>&1`)
	fmt.Println("\nSocket test:", out)
}
