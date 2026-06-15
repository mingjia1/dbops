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

	// Check host 16 and 17
	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("=== %s: SSH fail: %v ===\n", host, err)
			continue
		}

		fmt.Printf("\n=== %s ===\n", host)
		// Test connection as repl user (which we know works)
		out := runCmd(client, `mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT 1 AS alive;" 2>&1`, 5)
		fmt.Println("MySQL alive:", out)

		// Check MGR status
		out = runCmd(client, `mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SELECT member_host, member_port, member_state, member_role FROM performance_schema.replication_group_members;" 2>&1`, 5)
		fmt.Println("MGR members:", out)

		// Check if GR port is listening
		out = runCmd(client, `ss -tlnp | grep 3306`, 5)
		fmt.Println("Ports:", out)

		client.Close()
	}
}
