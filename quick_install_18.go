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

	// 检查是否已有xtrabackup
	session1, _ := client.NewSession()
	out1, _ := session1.CombinedOutput("which xtrabackup 2>&1")
	session1.Close()
	if len(out1) > 0 && string(out1) != "" {
		fmt.Printf("xtrabackup already installed: %s\n", string(out1))
		return
	}

	// 安装percona-xtrabackup-24
	fmt.Println("Installing percona-xtrabackup-24...")
	session2, _ := client.NewSession()
	cmd := `export DEBIAN_FRONTEND=noninteractive && apt-get install -y percona-xtrabackup-24 2>&1`
	out2, err2 := session2.CombinedOutput(cmd)
	session2.Close()

	fmt.Printf("Install output (last 50 lines):\n")
	lines := string(out2)
	if len(lines) > 5000 {
		lines = lines[len(lines)-5000:]
	}
	fmt.Println(lines)

	if err2 != nil {
		fmt.Printf("Install error: %v\n", err2)
	}

	// 验证
	session3, _ := client.NewSession()
	out3, _ := session3.CombinedOutput("which xtrabackup && xtrabackup --version 2>&1 | head -1")
	session3.Close()
	fmt.Printf("\nVerification:\n%s\n", string(out3))
}
