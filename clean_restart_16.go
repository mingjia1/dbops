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

	client, _ := ssh.Dial("tcp", "10.1.81.16:22", config)
	defer client.Close()

	// 1. 查看所有agent相关进程
	fmt.Println("[1] All agent processes:")
	session1, _ := client.NewSession()
	out1, _ := session1.CombinedOutput("ps aux | grep -E 'agent' | grep -v grep")
	session1.Close()
	fmt.Printf("%s\n", string(out1))

	// 2. 查看9090端口占用
	fmt.Println("[2] Port 9090 usage:")
	session2, _ := client.NewSession()
	out2, _ := session2.CombinedOutput("ss -tlnp | grep :9090")
	session2.Close()
	fmt.Printf("%s\n", string(out2))

	// 3. 彻底杀死所有agent进程
	fmt.Println("[3] Killing all agents...")
	session3, _ := client.NewSession()
	session3.CombinedOutput("pkill -9 -f 'agent'; pkill -9 -f 'dbops-agent'; sleep 2")
	session3.Close()
	time.Sleep(3 * time.Second)

	// 4. 确认端口释放
	fmt.Println("[4] Port after kill:")
	session4, _ := client.NewSession()
	out4, _ := session4.CombinedOutput("ss -tlnp | grep :9090 || echo 'Port free'")
	session4.Close()
	fmt.Printf("%s\n", string(out4))

	// 5. 启动新Agent
	fmt.Println("[5] Starting new agent...")
	session5, _ := client.NewSession()
	session5.CombinedOutput("cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 &")
	session5.Close()
	time.Sleep(4 * time.Second)

	// 6. 验证只有一个新Agent
	fmt.Println("[6] Final agent process:")
	session6, _ := client.NewSession()
	out6, _ := session6.CombinedOutput("ps aux | grep agent-new | grep -v grep | awk '{print $2, $9}'")
	session6.Close()
	fmt.Printf("%s\n", string(out6))

	// 7. 验证健康
	fmt.Println("[7] Health check:")
	session7, _ := client.NewSession()
	out7, _ := session7.CombinedOutput("curl -s http://localhost:9090/health")
	session7.Close()
	fmt.Printf("%s\n", string(out7))
}
