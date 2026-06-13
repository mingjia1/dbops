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
		"cd /opt/dbops-agent && ls -la",
		"cd /opt/dbops-agent && cat config/config.yaml 2>/dev/null || echo 'No config'",
		"cd /opt/dbops-agent && tail -50 agent.log 2>/dev/null || echo 'No log'",
		"cd /opt/dbops-agent && ./agent &",
	}

	for i, cmd := range commands[:3] {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}

	// 尝试启动
	fmt.Println("\n[4] Starting agent...")
	session4, _ := client.NewSession()
	output4, err4 := session4.CombinedOutput("cd /opt/dbops-agent && nohup ./agent > /tmp/agent-start.log 2>&1 & sleep 2 && cat /tmp/agent-start.log")
	session4.Close()
	if err4 != nil {
		fmt.Printf("Error: %v\n", err4)
	}
	fmt.Printf("Output:\n%s\n", string(output4))
}
