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

		// List agent-related processes
		out := runCmd(client, `ps aux | grep -E "(agent|mysql-ops)" | grep -v grep`, 5)
		fmt.Printf("Agent processes:\n%s\n", out)

		// Check if agent binary exists
		out = runCmd(client, `ls -la /data/mgr-agent/ 2>/dev/null; ls -la /opt/mgr-agent/ 2>/dev/null; ls -la /root/agent/ 2>/dev/null; which agent_linux 2>/dev/null; find / -name "agent_linux" -o -name "mgr-agent" 2>/dev/null | head -5`, 10)
		fmt.Printf("Agent binaries:\n%s\n", out)

		// Check if config exists anywhere
		out = runCmd(client, `find / -name "config.yaml" -path "*/agent*" 2>/dev/null | head -5`, 10)
		fmt.Printf("Config files:\n%s\n", out)

		client.Close()
	}
}
