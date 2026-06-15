package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func main() {
	sshUser, sshPass := "root", "hcfc!2017"
	hosts := []string{"10.1.81.16", "10.1.81.17", "10.1.81.18"}
	// Test empty password and the page password
	passwords := []string{"", "Hcfc@DboOps#2024_80"}

	config := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	for _, host := range hosts {
		fmt.Printf("\n=== %s ===\n", host)
		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("SSH failed: %v\n", err)
			continue
		}

		for _, pwd := range passwords {
			label := fmt.Sprintf("pwd=%q", pwd)
			if pwd == "" {
				label = "pwd=\"\" (empty)"
			}
			session, err := client.NewSession()
			if err != nil {
				fmt.Printf("  %s: session err %v\n", label, err)
				continue
			}
			out, err := session.CombinedOutput(fmt.Sprintf("mysql -uroot -p'%s' -e 'SELECT 1 AS test;' 2>&1", pwd))
			if err != nil {
				fmt.Printf("  %s: FAIL - %s\n", label, strings.TrimSpace(string(out)))
			} else {
				fmt.Printf("  %s: OK - %s", label, strings.TrimSpace(string(out)))
			}
			session.Close()
		}
		client.Close()
	}
}
