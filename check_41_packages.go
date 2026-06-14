package main

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/ssh"
)

func runOn41(config *ssh.ClientConfig, cmd string) {
	session, err := ssh.Dial("tcp", "10.1.81.41:22", config)
	if err != nil {
		log.Printf("Connect failed: %v", err)
		return
	}
	defer session.Close()

	s, err := session.NewSession()
	if err != nil {
		log.Printf("Session error: %v", err)
		return
	}
	defer s.Close()

	out, err := s.CombinedOutput(cmd)
	fmt.Printf("Output:\n%s\n", string(out))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Check current state on 41
	runOn41(config, "echo '--- deb/ ---' && ls -lh /opt/packet/deb/ 2>/dev/null || echo empty")
	runOn41(config, "echo '--- tarball/ ---' && ls -lh /opt/packet/tarball/ 2>/dev/null || echo empty")
	runOn41(config, "echo '--- root /opt/packet/ ---' && ls -lh /opt/packet/*.deb /opt/packet/*.gz 2>/dev/null || echo 'no files in root'")
	runOn41(config, "df -h /")
}
