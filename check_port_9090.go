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

	hosts := []string{"10.1.81.16", "10.1.81.17"}

	for _, host := range hosts {
		fmt.Printf("\n=== Checking %s ===\n", host)

		client, _ := ssh.Dial("tcp", host+":22", config)

		commands := []string{
			"netstat -tlnp 2>/dev/null | grep :9090 || ss -tlnp | grep :9090",
			"curl -s http://localhost:9090/health 2>&1 | head -3",
		}

		for _, cmd := range commands {
			fmt.Printf("$ %s\n", cmd)
			session, _ := client.NewSession()
			output, _ := session.CombinedOutput(cmd)
			session.Close()
			fmt.Printf("%s\n", string(output))
		}

		client.Close()
	}
}
