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
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", "10.1.81.18:22", config)
	if err != nil {
		fmt.Printf("Dial failed: %v\n", err)
		return
	}
	defer client.Close()

	// 检查运行中的apt进程
	fmt.Println("Checking running apt processes...")
	session1, _ := client.NewSession()
	out1, _ := session1.CombinedOutput("ps aux | grep apt | grep -v grep")
	session1.Close()
	fmt.Printf("Running apt processes:\n%s\n", string(out1))

	// 等待锁释放（最多60秒）
	fmt.Println("\nWaiting for apt lock to be released (max 60s)...")
	for i := 0; i < 12; i++ {
		session2, _ := client.NewSession()
		out2, _ := session2.CombinedOutput("fuser /var/lib/dpkg/lock-frontend 2>&1")
		session2.Close()

		if len(out2) == 0 || string(out2) == "" {
			fmt.Println("Lock released!")
			break
		}
		fmt.Printf("  [%ds] Still locked by: %s", (i+1)*5, string(out2))
		time.Sleep(5 * time.Second)
	}

	// 检查xtrabackup
	fmt.Println("\nChecking xtrabackup...")
	session3, _ := client.NewSession()
	out3, _ := session3.CombinedOutput("which xtrabackup && xtrabackup --version 2>&1 | head -3")
	session3.Close()
	fmt.Printf("Result:\n%s\n", string(out3))
}
