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

		// Check config directory
		out := runCmd(client, `ls -la /opt/dbops-agent/config/ 2>/dev/null`, 5)
		fmt.Printf("Config dir:\n%s\n", out)

		out = runCmd(client, `cat /opt/dbops-agent/config/*.yaml /opt/dbops-agent/config/*.yml 2>/dev/null || echo "no yaml found"`, 5)
		fmt.Printf("Config file content:\n%s\n", out)

		out = runCmd(client, `cat /opt/dbops-agent/config.yaml 2>/dev/null || echo "no config.yaml"`, 5)
		fmt.Printf("/opt/dbops-agent/config.yaml:\n%s\n", out)

		client.Close()
	}
}
