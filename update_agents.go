package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"time"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// 读取新编译的Agent
	agentBinary, err := ioutil.ReadFile("d:/test_tmple/new_dbops/dbops/agent/bin/agent_linux")
	if err != nil {
		fmt.Printf("Failed to read agent binary: %v\n", err)
		return
	}
	fmt.Printf("Agent binary size: %d bytes\n", len(agentBinary))

	hosts := []string{"10.1.81.16", "10.1.81.17"}

	for _, host := range hosts {
		fmt.Printf("\n=== Updating Agent on %s ===\n", host)

		client, err := ssh.Dial("tcp", host+":22", config)
		if err != nil {
			fmt.Printf("Failed to connect: %v\n", err)
			continue
		}

		// 停止旧Agent
		session1, _ := client.NewSession()
		session1.CombinedOutput("pkill -f 'agent-new|dbops-agent'")
		session1.Close()
		time.Sleep(2 * time.Second)

		// 上传新Agent
		session2, _ := client.NewSession()
		uploadCmd := fmt.Sprintf("cat > /opt/dbops-agent/agent-new && chmod +x /opt/dbops-agent/agent-new")
		stdin, _ := session2.StdinPipe()
		session2.Start(uploadCmd)
		stdin.Write(agentBinary)
		stdin.Close()
		session2.Wait()
		session2.Close()
		fmt.Println("✓ Agent binary uploaded")

		// 启动新Agent
		session3, _ := client.NewSession()
		startCmd := "cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 &"
		session3.CombinedOutput(startCmd)
		session3.Close()
		fmt.Println("✓ Agent started")

		client.Close()
		time.Sleep(3 * time.Second)

		// 验证
		client2, _ := ssh.Dial("tcp", host+":22", config)
		session4, _ := client2.NewSession()
		out, _ := session4.CombinedOutput("ps aux | grep agent-new | grep -v grep | wc -l")
		session4.Close()
		client2.Close()

		if string(out)[0] == '1' {
			fmt.Printf("✅ Agent running on %s\n", host)
		} else {
			fmt.Printf("❌ Agent not running on %s\n", host)
		}
	}

	fmt.Println("\n=== Verification ===")
	time.Sleep(2 * time.Second)
	for _, host := range hosts {
		fmt.Printf("Checking %s health...\n", host)
		// Will check via HTTP in next step
	}
}
