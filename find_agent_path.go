package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, _ := ssh.Dial("tcp", "10.1.81.16:22", config)
	defer client.Close()

	commands := []string{
		"ps aux | grep mysql-ops-agent | grep -v grep",
		"ls -la /opt/ | grep -i agent",
		"find /opt -name agent 2>/dev/null | head -5",
		"which agent 2>/dev/null || echo 'agent not in PATH'",
	}

	for _, cmd := range commands {
		fmt.Printf("\n$ %s\n", cmd)
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
