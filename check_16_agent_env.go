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

	client, err := ssh.Dial("tcp", "10.1.81.16:22", config)
	if err != nil {
		fmt.Printf("Dial failed: %v\n", err)
		return
	}
	defer client.Close()

	commands := []string{
		"which mysql",
		"mysql --version",
		"ps aux | grep mysql-ops-agent | grep -v grep",
		"cat /proc/$(pgrep -f mysql-ops-agent | head -1)/environ | tr '\\0' '\\n' | grep PATH",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
