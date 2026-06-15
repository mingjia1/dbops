package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Only do this on host 16 (PRIMARY)
	host := "10.1.81.16"
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Printf("SSH fail: %v\n", err)
		return
	}
	defer client.Close()

	cmds := []string{
		// Try to find the root password or reset it safely
		`sudo cat /data/mysql/3306/error.log | head -5`,

		// Check if there's a debian.cnf from the package
		`find /etc -name "debian.cnf" -exec cat {} \; 2>/dev/null || echo "not found"`,

		// Check mysql history files
		`cat ~/.mysql_history 2>/dev/null | tail -20 || echo "no history"`,
		`cat /root/.mysql_history 2>/dev/null | tail -20 || echo "no root history"`,

		// Try to find any config with password
		`find /etc/mysql /root /home -name "*.cnf" -o -name "*.my" 2>/dev/null | head -10`,
		`grep -r "password\|PASSWORD\|pwd" /etc/mysql/ 2>/dev/null | head -10`,

		// Check for .mylogin.cnf
		`ls -la /root/.mylogin.cnf 2>/dev/null || echo "no mylogin.cnf"`,
	}

	for _, cmd := range cmds {
		session, err := client.NewSession()
		if err != nil {
			fmt.Printf("ERROR: session: %v\n", err)
			continue
		}
		out, err := session.CombinedOutput(cmd)
		if err != nil {
			fmt.Printf("FAIL [%s]: %s\n", cmd, strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("OK [%s]: %s\n", cmd, strings.TrimSpace(string(out)))
		}
		session.Close()
	}
}
