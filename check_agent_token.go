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

		// Check agent config file
		out := runCmd(client, `cat /data/mgr-agent/config.yaml 2>/dev/null || cat /opt/mgr-agent/config.yaml 2>/dev/null || cat /root/agent/config.yaml 2>/dev/null || echo "no config.yaml found in common paths"`, 5)
		fmt.Printf("Config: %s\n", out)

		// Check for env var
		out = runCmd(client, `cat /proc/$(pgrep -f "mysql-ops-agent" 2>/dev/null)/environ 2>/dev/null | tr '\0' '\n' | grep -i agent || echo "no agent proc found"`, 5)
		fmt.Printf("Agent env: %s\n", out)

		// Check agent process cmdline
		out = runCmd(client, `cat /proc/$(pgrep -f "mysql-ops-agent" 2>/dev/null)/cmdline 2>/dev/null | tr '\0' ' ' || echo "no agent proc found"`, 5)
		fmt.Printf("Agent cmdline: %s\n", out)

		client.Close()
	}
}
