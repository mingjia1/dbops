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

	for _, host := range []string{"10.1.81.16"} {
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("%s: SSH fail\n", host)
			continue
		}
		fmt.Printf("=== %s ===\n", host)

		// Check stdout/stderr logs
		out := runCmd(client, `cat /opt/dbops-agent/stdout.log 2>/dev/null | tail -20`, 5)
		fmt.Printf("stdout.log:\n%s\n", out)

		out = runCmd(client, `cat /opt/dbops-agent/stderr.log 2>/dev/null | tail -20`, 5)
		fmt.Printf("stderr.log:\n%s\n", out)

		// List files in agent directory
		out = runCmd(client, `ls -la /opt/dbops-agent/ 2>/dev/null`, 5)
		fmt.Printf("Agent dir:\n%s\n", out)

		client.Close()
		break // just check host 16 for now
	}
}
