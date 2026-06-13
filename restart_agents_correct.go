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

		client, _ := ssh.Dial("tcp", host+":22", config)

		// 查找Agent路径
		session0, _ := client.NewSession()
		pathOut, _ := session0.CombinedOutput("find /opt -name agent -type f 2>/dev/null | head -1")
		session0.Close()
		agentPath := string(pathOut)
		if len(agentPath) == 0 {
			fmt.Printf("Agent not found on %s\n", host)
			client.Close()
			continue
		}
		agentPath = agentPath[:len(agentPath)-1] // trim newline
		fmt.Printf("Agent path: %s\n", agentPath)

		// 停止
		session1, _ := client.NewSession()
		session1.CombinedOutput("pkill -f dbops-agent")
		session1.Close()
		time.Sleep(2 * time.Second)

		// 启动
		session2, _ := client.NewSession()
		cmd := fmt.Sprintf("cd %s/.. && nohup ./agent > agent.log 2>&1 &", agentPath[:len(agentPath)-6])
		fmt.Printf("Start command: %s\n", cmd)
		session2.CombinedOutput(cmd)
		session2.Close()

		client.Close()
		time.Sleep(3 * time.Second)

		// 验证
		client2, _ := ssh.Dial("tcp", host+":22", config)
		session3, _ := client2.NewSession()
		out3, _ := session3.CombinedOutput("ps aux | grep dbops-agent | grep -v grep")
		session3.Close()
		client2.Close()

		if len(out3) > 0 {
			fmt.Printf("✓ Agent running\n")
		} else {
			fmt.Printf("✗ Agent not running\n")
		}
	}
}
