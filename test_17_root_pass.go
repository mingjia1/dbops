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

	pass := "Hcfc@DboOps#2024_80"

	client, err := ssh.Dial("tcp", "10.1.81.17:22", config)
	if err != nil {
		fmt.Printf("SSH: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("=== Test root password on host 17 ===")
	out := runCmd(client, fmt.Sprintf(`mysql -h127.0.0.1 -P3306 -uroot -p'%s' -e "SELECT 1 AS test;" 2>&1`, pass), 5)
	if strings.Contains(out, "test") {
		fmt.Println("PASSWORD OK! Host 17 also uses Hcfc@DboOps#2024_80")
	} else {
		fmt.Printf("FAIL: %s\n", out)
	}

	// Check repl user's privileges
	out = runCmd(client, `mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "SHOW GRANTS;" 2>&1`, 5)
	fmt.Println("\nRepl grants:", out)

	// Try to START GROUP_REPLICATION as repl
	out = runCmd(client, `mysql -h127.0.0.1 -P3306 -urepl -prepl_123 -e "START GROUP_REPLICATION;" 2>&1`, 10)
	fmt.Println("\nSTART GR as repl:", out)
}
