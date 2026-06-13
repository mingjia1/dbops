package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"time"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	hosts := []string{"10.1.81.16", "10.1.81.17"}

	for _, host := range hosts {
		fmt.Printf("\n=== Restarting Agent on %s ===\n", host)

		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("Skip %s: %v\n", host, err)
			continue
		}

		// 停止Agent
		session1, _ := client.NewSession()
		session1.CombinedOutput("pkill -f mysql-ops-agent")
		session1.Close()

		time.Sleep(2 * time.Second)

		// 启动Agent
		session2, _ := client.NewSession()
		cmd := `cd /opt/mysql-ops-agent && nohup ./agent > agent.log 2>&1 &`
		output, _ := session2.CombinedOutput(cmd)
		session2.Close()
		fmt.Printf("Start output: %s\n", string(output))

		client.Close()

		time.Sleep(3 * time.Second)

		// 验证
		fmt.Printf("Verifying %s...\n", host)
		client2, _ := ssh.Dial("tcp", host+":22", config)
		session3, _ := client2.NewSession()
		out3, _ := session3.CombinedOutput("ps aux | grep mysql-ops-agent | grep -v grep")
		session3.Close()
		client2.Close()

		fmt.Printf("Agent process: %s\n", string(out3))
	}
}
