package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("hcfc!2017"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", "10.1.81.41:22", config)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// 上传脚本
	scriptContent, err := ioutil.ReadFile("../setup_local_repo.sh")
	if err != nil {
		log.Fatalf("Failed to read script: %v", err)
	}

	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// 写入脚本到远程
	cmd := fmt.Sprintf("cat > /tmp/setup_local_repo.sh << 'EOFSCRIPT'\n%s\nEOFSCRIPT\nchmod +x /tmp/setup_local_repo.sh\nbash /tmp/setup_local_repo.sh", string(scriptContent))

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		fmt.Printf("Command output:\n%s\n", string(output))
		log.Fatalf("Failed to execute: %v", err)
	}

	fmt.Printf("=== Execution Output ===\n%s\n", string(output))
	os.Exit(0)
}
