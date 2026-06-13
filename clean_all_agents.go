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

	for _, host := range []string{"10.1.81.16", "10.1.81.17"} {
		fmt.Printf("\n=== Cleaning %s ===\n", host)
		client, _ := ssh.Dial("tcp", host+":22", config)

		// 1. Kill ALL agent processes
		session1, _ := client.NewSession()
		out1, _ := session1.CombinedOutput("pkill -9 -f agent; pkill -9 -f dbops; sleep 2; ps aux | grep -E 'agent|dbops' | grep -v grep || echo 'All clean'")
		session1.Close()
		fmt.Printf("Kill: %s\n", string(out1))

		// 2. Check port 9090
		session2, _ := client.NewSession()
		out2, _ := session2.CombinedOutput("ss -tlnp | grep :9090 || echo 'Port 9090 free'")
		session2.Close()
		fmt.Printf("Port: %s\n", string(out2))

		// 3. Verify agent binary MD5
		session3, _ := client.NewSession()
		out3, _ := session3.CombinedOutput("md5sum /opt/dbops-agent/agent-new | awk '{print $1}'")
		session3.Close()
		fmt.Printf("MD5: %s", string(out3))

		// 4. Start fresh agent
		session4, _ := client.NewSession()
		session4.CombinedOutput("cd /opt/dbops-agent && nohup ./agent-new > agent-clean.log 2>&1 &")
		session4.Close()

		client.Close()
		time.Sleep(4 * time.Second)

		// 5. Verify
		client2, _ := ssh.Dial("tcp", host+":22", config)
		session5, _ := client2.NewSession()
		out5, _ := session5.CombinedOutput("curl -s http://localhost:9090/health | head -1")
		session5.Close()
		client2.Close()

		fmt.Printf("Health: %s\n", string(out5))
	}

	fmt.Println("\n✅ Both agents cleaned and restarted")
}
