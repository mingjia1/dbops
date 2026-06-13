package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"net/http"
	"time"
)

func updateHost(host string, agentBinary []byte) {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	fmt.Printf("=== Updating %s ===\n", host)

	client, _ := ssh.Dial("tcp", host+":22", config)
	defer client.Close()

	// Stop
	session1, _ := client.NewSession()
	session1.CombinedOutput("pkill -f agent-new")
	session1.Close()
	time.Sleep(1 * time.Second)

	// Upload
	session2, _ := client.NewSession()
	stdin, _ := session2.StdinPipe()
	session2.Start("cat > /opt/dbops-agent/agent-new && chmod +x /opt/dbops-agent/agent-new")
	stdin.Write(agentBinary)
	stdin.Close()
	session2.Wait()
	session2.Close()

	// Start
	session3, _ := client.NewSession()
	session3.CombinedOutput("cd /opt/dbops-agent && nohup ./agent-new > agent-new.log 2>&1 &")
	session3.Close()

	fmt.Printf("  ✓ Agent updated\n")
}

func main() {
	agentBinary, _ := ioutil.ReadFile("d:/test_tmple/new_dbops/dbops/agent/bin/agent_linux")
	fmt.Printf("Agent binary: %d bytes\n\n", len(agentBinary))

	// Update both hosts
	updateHost("10.1.81.16", agentBinary)
	updateHost("10.1.81.17", agentBinary)

	// Wait and verify
	fmt.Println("\nWaiting 5 seconds...")
	time.Sleep(5 * time.Second)

	fmt.Println("\n=== Verification ===")
	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		resp, err := http.Get(fmt.Sprintf("http://%s:9090/health", host))
		if err != nil {
			fmt.Printf("%s: ❌ %v\n", host, err)
		} else {
			resp.Body.Close()
			fmt.Printf("%s: ✅ Agent healthy\n", host)
		}
	}
}
