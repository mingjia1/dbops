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
	fmt.Printf("Agent size: %d bytes, md5 should be: 7080b05717c0f3dd22d1917dd57c4ea6\n\n", len(agentBinary))

	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		fmt.Printf("=== Updating %s ===\n", host)

		client, _ := ssh.Dial("tcp", host+":22", config)

		// Kill all agent processes
		fmt.Println("  Stopping all agents...")
		session1, _ := client.NewSession()
		out1, _ := session1.CombinedOutput("pkill -9 -f agent-new; sleep 1; ps aux | grep agent-new | grep -v grep || echo 'All stopped'")
		session1.Close()
		fmt.Printf("  %s\n", string(out1))

		// Upload
		fmt.Println("  Uploading...")
		session2, _ := client.NewSession()
		stdin, _ := session2.StdinPipe()
		session2.Start("cat > /opt/dbops-agent/agent-new.tmp && mv /opt/dbops-agent/agent-new.tmp /opt/dbops-agent/agent-new && chmod +x /opt/dbops-agent/agent-new")
		stdin.Write(agentBinary)
		stdin.Close()
		session2.Wait()
		session2.Close()

		// Verify upload
		session3, _ := client.NewSession()
		out3, _ := session3.CombinedOutput("md5sum /opt/dbops-agent/agent-new")
		session3.Close()
		fmt.Printf("  MD5: %s", string(out3))

		// Start
		fmt.Println("  Starting...")
		session4, _ := client.NewSession()
		session4.CombinedOutput("cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 &")
		session4.Close()

		client.Close()
		time.Sleep(2 * time.Second)

		// Verify process
		client2, _ := ssh.Dial("tcp", host+":22", config)
		session5, _ := client2.NewSession()
		out5, _ := session5.CombinedOutput("ps aux | grep agent-new | grep -v grep | wc -l")
		session5.Close()
		client2.Close()

		if string(out5)[0] == '1' {
			fmt.Printf("  ✅ Done\n\n")
		} else {
			fmt.Printf("  ⚠️ Process count: %s\n", string(out5))
		}
	}

	fmt.Println("=== Final Verification ===")
	time.Sleep(3 * time.Second)
	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		client, _ := ssh.Dial("tcp", host+":22", config)
		session, _ := client.NewSession()
		out, _ := session.CombinedOutput("curl -s http://localhost:9090/health | head -1")
		session.Close()
		client.Close()
		fmt.Printf("%s: %s\n", host, string(out))
	}
}
