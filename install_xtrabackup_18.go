package main

import (
	"fmt"
	"log"
	"golang.org/x/crypto/ssh"
)

func main() {
	hosts := []string{"10.1.81.18"}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	for _, host := range hosts {
		fmt.Printf("\n=== Working on %s ===\n", host)

		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("Skip %s: %v\n", host, err)
			continue
		}

		session, _ := client.NewSession()
		cmd := `
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y percona-xtrabackup-24 2>&1 | tail -30
which xtrabackup
xtrabackup --version 2>&1 | head -1
`
		output, err := session.CombinedOutput(cmd)
		session.Close()
		client.Close()

		fmt.Printf("Output:\n%s\n", string(output))
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}
