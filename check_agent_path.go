package main

import (
	"fmt"
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

	for _, host := range []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"} {
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("[%s] CONNECT FAILED: %v\n", host, err)
			continue
		}

		// Check agent PID and env
		session, _ := client.NewSession()
		out, _ := session.CombinedOutput("ps aux | grep '[a]gent' | awk '{print $2}' | head -1")
		pid := string(out)
		session.Close()

		session, _ = client.NewSession()
		cmd := fmt.Sprintf("cat /proc/%s/environ 2>/dev/null | tr '\\0' '\\n'", pid)
		out, _ = session.CombinedOutput(cmd)
		session.Close()

		fmt.Printf("[%s] PID=%sPATH:\n", host, pid)
		for _, line := range splitLines(string(out)) {
			if contains(line, "PATH") || contains(line, "mysql") {
				fmt.Printf("  %s\n", line)
			}
		}

		// Also check if mysql is reachable via agent
		session, _ = client.NewSession()
		out, _ = session.CombinedOutput("su -s /bin/bash nobody -c 'which mysql' 2>/dev/null || echo 'NOT_FOUND'")
		session.Close()
		fmt.Printf("[%s] nobody user mysql: %s", host, string(out))

		client.Close()
	}
}

func splitLines(s string) []string {
	var lines []string
	line := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(c)
		}
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
