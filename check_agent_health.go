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
		return "ERR"
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

	for _, host := range []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"} {
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("%s: SSH fail\n", host)
			continue
		}
		fmt.Printf("\n=== %s ===\n", host)

		// Find agent PID
		out := runCmd(client, `pgrep -f "^\./agent$" || pgrep -f "dbops-agent" | head -1`, 5)
		pid := strings.TrimSpace(out)
		fmt.Printf("Agent PID: %s\n", pid)

		if pid != "" && pid != "ERR" && pid != "TIMEOUT" {
			// Check environment vars for the agent
			out = runCmd(client, fmt.Sprintf(`cat /proc/%s/environ 2>/dev/null | tr '\0' '\n' | grep -iE "agent|token|pass"`, pid), 5)
			fmt.Printf("Agent env vars:\n%s\n", out)
		}

		// Check agent health endpoint
		out = runCmd(client, `curl -s http://127.0.0.1:9090/health 2>/dev/null || echo "FAILED"`, 5)
		fmt.Printf("Agent /health: %s\n", out)

		// Try agent tasks without token
		out = runCmd(client, `curl -s -X POST http://127.0.0.1:9090/agent/tasks/health-check -H "Content-Type: application/json" -d '{"instance_id":"test"}' 2>/dev/null || echo "FAILED"`, 5)
		fmt.Printf("Agent tasks (no token): %s\n", out)

		// Try with correct token
		out = runCmd(client, `curl -s -X POST http://127.0.0.1:9090/agent/tasks/health-check -H "Content-Type: application/json" -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" -d '{"instance_id":"test"}' 2>/dev/null || echo "FAILED"`, 5)
		fmt.Printf("Agent tasks (with token): %s\n", out)

		client.Close()
	}
}
