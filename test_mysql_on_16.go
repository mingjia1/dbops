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

	tests := []string{
		// 测试mysql命令
		"which mysql",
		"/usr/bin/mysql --version",

		// 测试Agent进程环境
		"cat /proc/$(pgrep -f agent-new | head -1)/environ | tr '\\0' '\\n' | grep PATH",

		// 测试从Agent进程同样的环境执行mysql
		"cd /opt/dbops-agent && ./agent-new version 2>&1 || echo 'No version command'",

		// 检查Agent日志中的最新错误
		"tail -100 /opt/dbops-agent/agent-new.log | grep -i 'mysql\\|error' | tail -20",

		// 测试通过Agent API调用mysql相关操作
		`curl -s http://localhost:9090/agent/tasks/check-tools \
		  -H "Authorization: Bearer dev-agent-token-CHANGE-ME-at-least-16" | head -5`,
	}

	for i, cmd := range tests {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		fmt.Println("---")
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
