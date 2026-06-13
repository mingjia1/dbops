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

	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		fmt.Printf("\n=== %s ===\n", host)

		client, _ := ssh.Dial("tcp", host+":22", config)

		commands := []string{
			"ps aux | grep agent-new | grep -v grep | awk '{print $2,$9}'",
			"ls -lh /opt/dbops-agent/agent-new | awk '{print $5,$6,$7,$8,$9}'",
			"md5sum /opt/dbops-agent/agent-new",
			"strings /opt/dbops-agent/agent-new | grep -i 'getMySQLBinary' | head -1",
		}

		for _, cmd := range commands {
			session, _ := client.NewSession()
			output, _ := session.CombinedOutput(cmd)
			session.Close()
			fmt.Printf("%s\n", string(output))
		}

		client.Close()
	}
}
