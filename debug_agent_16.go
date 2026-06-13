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
		// 检查Agent进程
		"ps aux | grep agent-new | grep -v grep",

		// 检查Agent二进制更新时间
		"ls -lh /opt/dbops-agent/agent-new",

		// 检查mysql命令
		"which mysql",
		"ls -l /usr/bin/mysql",

		// 检查Agent日志最后50行
		"tail -50 /opt/dbops-agent/agent-new.log",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		fmt.Println("---")
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
