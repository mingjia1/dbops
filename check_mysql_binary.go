package main

import (
	"fmt"
	"os"
	"os/exec"
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

		// Check agent PID environment
		session, _ := client.NewSession()
		out, _ := session.CombinedOutput("ps aux | grep '[a]gent' | grep -v 'bash -c' | grep -v vscode | awk '{print $2}' | head -1")
		pid := trimSpace(string(out))
		session.Close()

		if pid == "" {
			fmt.Printf("[%s] No agent PID found\n", host)
			client.Close()
			continue
		}

		session, _ = client.NewSession()
		out, _ = session.CombinedOutput(fmt.Sprintf("cat /proc/%s/environ 2>/dev/null | tr '\\0' '\\n'", pid))
		envStr := string(out)
		session.Close()

		fmt.Printf("[%s] Agent PID=%s\n", host, pid)
		for _, line := range splitLines(envStr) {
			if contains(line, "PATH") || contains(line, "HOME") || contains(line, "PWD") {
				fmt.Printf("  %s\n", line)
			}
		}

		// Test os.Stat /usr/bin/mysql directly via Go-like approach
		session, _ = client.NewSession()
		out, _ = session.CombinedOutput("test -x /usr/bin/mysql && echo 'EXISTS' || echo 'NOT_EXISTS'")
		session.Close()
		fmt.Printf("[%s] /usr/bin/mysql check: %s", host, string(out))

		// Try running mysql via PATH
		session, _ = client.NewSession()
		out, _ = session.CombinedOutput("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin which mysql 2>&1")
		session.Close()
		fmt.Printf("[%s] PATH which mysql: %s", host, string(out))

		client.Close()
	}
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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
	if line != "" {
		lines = append(lines, line)
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
