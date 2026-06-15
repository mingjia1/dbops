package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func runCmd(client *ssh.Client, cmd string, timeoutSec int) string {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Sprintf("SESSION ERR: %v", err)
	}
	defer s.Close()

	ch := make(chan string, 1)
	go func() {
		out, _ := s.CombinedOutput(cmd)
		ch <- strings.TrimSpace(string(out))
	}()

	select {
	case res := <-ch:
		return res
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		s.Close()
		return "TIMEOUT"
	}
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	pass := "Hcfc@DboOps#2024_80"

	client, err := ssh.Dial("tcp", "10.1.81.18:22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("=== Check MGR status after join attempt ===")

	// Check MGR member status
	out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, pass), 10)
	fmt.Println("MGR members:", out)

	// Check error log for GR errors
	out = runCmd(client, `grep -i "group_replication\|recovery\|error\|ERROR" /data/mysql/3306/error.log | tail -20`, 10)
	fmt.Println("\nError log (last 20 lines):")
	fmt.Println(out)

	// Check if mysql is still running
	out = runCmd(client, `pgrep -f "mysqld.*datadir=/data/mysql/3306" || echo "not running"`, 5)
	fmt.Println("\nMySQL PID:", out)

	// Try a simple connection test
	out = runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT 1 AS alive;" 2>&1`, pass), 5)
	fmt.Println("\nConnection test:", out)
}
