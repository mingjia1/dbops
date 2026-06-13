package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"net/http"
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
		fmt.Printf("\n=== %s ===\n", host)

		client, _ := ssh.Dial("tcp", host+":22", config)

		// 检查进程
		session1, _ := client.NewSession()
		out1, _ := session1.CombinedOutput("ps aux | grep agent-new | grep -v grep")
		session1.Close()
		fmt.Printf("Process: %s\n", string(out1))

		// 检查二进制
		session2, _ := client.NewSession()
		out2, _ := session2.CombinedOutput("ls -lh /opt/dbops-agent/agent-new && md5sum /opt/dbops-agent/agent-new")
		session2.Close()
		fmt.Printf("Binary: %s\n", string(out2))

		client.Close()

		// 检查健康
		time.Sleep(1 * time.Second)
		resp, err := http.Get(fmt.Sprintf("http://%s:9090/health", host))
		if err != nil {
			fmt.Printf("Health: ❌ %v\n", err)
		} else {
			defer resp.Body.Close()
			fmt.Printf("Health: ✅ OK\n")
		}
	}
}
