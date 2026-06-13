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
		// 查看Agent进程启动时间
		"ps aux | grep agent-new | grep -v grep | awk '{print $2,$9}'",

		// 查看最新Agent二进制
		"stat /opt/dbops-agent/agent-new | grep Modify",

		// 重启Agent并观察启动
		"pkill -9 agent-new; sleep 2; cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 & sleep 3; ps aux | grep agent-new | grep -v grep",

		// 验证新Agent健康
		"curl -s http://localhost:9090/health",

		// 查看新Agent日志
		"tail -30 /opt/dbops-agent/agent-new.log",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		fmt.Println("===")
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
