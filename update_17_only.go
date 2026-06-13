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

	agentBinary, _ := ioutil.ReadFile("d:/test_tmple/new_dbops/dbops/agent/bin/agent_linux")
	fmt.Printf("Agent size: %d bytes\n", len(agentBinary))

	host := "10.1.81.17"
	fmt.Printf("\n=== Updating %s ===\n", host)

	client, _ := ssh.Dial("tcp", host+":22", config)
	defer client.Close()

	// 停止
	fmt.Println("Stopping old agent...")
	session1, _ := client.NewSession()
	session1.CombinedOutput("pkill -f agent-new")
	session1.Close()
	time.Sleep(2 * time.Second)

	// 上传
	fmt.Println("Uploading new agent...")
	session2, _ := client.NewSession()
	stdin, _ := session2.StdinPipe()
	session2.Start("cat > /opt/dbops-agent/agent-new && chmod +x /opt/dbops-agent/agent-new")
	stdin.Write(agentBinary)
	stdin.Close()
	session2.Wait()
	session2.Close()

	// 启动
	fmt.Println("Starting new agent...")
	session3, _ := client.NewSession()
	session3.CombinedOutput("cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 &")
	session3.Close()

	time.Sleep(3 * time.Second)

	// 验证
	session4, _ := client.NewSession()
	out, _ := session4.CombinedOutput("ps aux | grep agent-new | grep -v grep && md5sum /opt/dbops-agent/agent-new")
	session4.Close()

	fmt.Printf("\nResult:\n%s\n", string(out))
	fmt.Println("✅ Done")
}
