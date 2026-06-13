package main

import (
	"fmt"
	"log"
	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", "10.1.81.41:22", config)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	commands := []string{
		"mkdir -p /opt/packet/{deb,rpm}",
		"ls -la /opt/packet",
		"cd /opt/packet/deb && wget -q -nc https://downloads.percona.com/downloads/Percona-XtraBackup-8.0/Percona-XtraBackup-8.0.35-31/binary/tarball/percona-xtrabackup-8.0.35-31-Linux-x86_64.glibc2.17.tar.gz || echo 'Download failed or exists'",
		"ls -lh /opt/packet/deb/",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d] Running: %s\n", i+1, cmd)
		session, _ := client.NewSession()
		output, err := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("Output:\n%s\n", string(output))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
