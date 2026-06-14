package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", "10.1.81.41:22", config)
	if err != nil {
		log.Fatalf("Failed to connect to 10.1.81.41: %v", err)
	}
	defer client.Close()

	commands := []string{
		// Check current state
		"echo '=== Host Info ===' && cat /etc/os-release 2>/dev/null | head -3 && uname -m",
		"echo '=== Current /opt/packet contents ===' && ls -lh /opt/packet/ 2>/dev/null || echo 'Directory /opt/packet not found'",

		// Create packet directory
		"mkdir -p /opt/packet/{deb,rpm,tarball}",

		// Download MySQL APT config
		"echo '=== Downloading MySQL APT config ==='",
		"cd /opt/packet/deb && if [ ! -f mysql-apt-config_0.8.33-1_all.deb ]; then wget -q -nc https://dev.mysql.com/get/mysql-apt-config_0.8.33-1_all.deb && echo 'Downloaded mysql-apt-config' || echo 'Failed mysql-apt-config'; else echo 'Already exists'; fi",
		"cd /opt/packet/deb && if [ ! -f mysql-apt-config_0.8.12-1_all.deb ]; then wget -q -nc https://repo.mysql.com/apt/ubuntu/pool/mysql-apt-config/m/mysql-apt-config/mysql-apt-config_0.8.12-1_all.deb && echo 'Downloaded mysql-apt-config 0.8.12' || echo 'Failed mysql-apt-config 0.8.12'; else echo 'Already exists'; fi",

		// Download Percona release config
		"echo '=== Downloading Percona release ==='",
		"cd /opt/packet/deb && if [ ! -f percona-release_latest.jammy_all.deb ]; then wget -q -nc https://repo.percona.com/apt/percona-release_latest.jammy_all.deb && echo 'Downloaded percona-release' || echo 'Failed percona-release'; else echo 'Already exists'; fi",

		// Download Percona XtraBackup 8.0 (for MySQL 8.0)
		"echo '=== Downloading XtraBackup 8.0 ==='",
		"cd /opt/packet/deb && if [ ! -f percona-xtrabackup-80_8.0.35-31-1.jammy_amd64.deb ]; then wget -q -nc https://downloads.percona.com/downloads/Percona-XtraBackup-8.0/Percona-XtraBackup-8.0.35-31/binary/debian/jammy/x86_64/percona-xtrabackup-80_8.0.35-31-1.jammy_amd64.deb && echo 'Downloaded xtrabackup-80' || echo 'Failed xtrabackup-80'; else echo 'Already exists'; fi",

		// Download Percona XtraBackup 2.4 (for MySQL 5.7)
		"echo '=== Downloading XtraBackup 2.4 ==='",
		"cd /opt/packet/tarball && if [ ! -f percona-xtrabackup-2.4.28-Linux-x86_64.glibc2.17.tar.gz ]; then wget -q -nc https://downloads.percona.com/downloads/Percona-XtraBackup-2.4/Percona-XtraBackup-2.4.28/binary/tarball/percona-xtrabackup-2.4.28-Linux-x86_64.glibc2.17.tar.gz && echo 'Downloaded xtrabackup-24' || echo 'Failed xtrabackup-24'; else echo 'Already exists'; fi",
		"cd /opt/packet/tarball && if [ ! -f percona-xtrabackup-8.0.35-31-Linux-x86_64.glibc2.17.tar.gz ]; then wget -q -nc https://downloads.percona.com/downloads/Percona-XtraBackup-8.0/Percona-XtraBackup-8.0.35-31/binary/tarball/percona-xtrabackup-8.0.35-31-Linux-x86_64.glibc2.17.tar.gz && echo 'Downloaded xtrabackup-80 tarball' || echo 'Failed xtrabackup-80 tarball'; else echo 'Already exists'; fi",

		// Try downloading via apt (for MySQL server packages if apt sources available)
		"echo '=== Trying apt-get download for MySQL packages ==='",
		"apt-get update -qq 2>/dev/null || true",
		"apt-get download mysql-server-5.7 mysql-client-5.7 2>/dev/null && mv *.deb /opt/packet/deb/ 2>/dev/null; echo 'apt 5.7 done'",
		"apt-get download mysql-server-8.0 mysql-client-8.0 2>/dev/null && mv *.deb /opt/packet/deb/ 2>/dev/null; echo 'apt 8.0 done'",

		// Download MySQL 8.0 tarball
		"echo '=== Downloading MySQL 8.0 tarball ==='",
		"cd /opt/packet/tarball && if [ ! -f mysql-8.0.36-linux-glibc2.17-x86_64-minimal.tar.xz ]; then wget -q -nc https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.36-linux-glibc2.17-x86_64-minimal.tar.xz && echo 'Downloaded mysql-8.0 tarball' || echo 'Failed mysql-8.0 tarball'; else echo 'Already exists'; fi",

		// Download MySQL 5.7 tarball
		"echo '=== Downloading MySQL 5.7 tarball ==='",
		"cd /opt/packet/tarball && if [ ! -f mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz ]; then wget -q -nc https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz && echo 'Downloaded mysql-5.7 tarball' || echo 'Failed mysql-5.7 tarball'; else echo 'Already exists'; fi",

		// Final listing
		"echo '' && echo '=== Final /opt/packet contents ==='",
		"echo '--- deb/ ---' && ls -lh /opt/packet/deb/ 2>/dev/null || echo 'empty'",
		"echo '--- tarball/ ---' && ls -lh /opt/packet/tarball/ 2>/dev/null || echo 'empty'",
		"echo '--- rpm/ ---' && ls -lh /opt/packet/rpm/ 2>/dev/null || echo 'empty'",
		"echo '--- Total size ---' && du -sh /opt/packet/ 2>/dev/null || true",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d/34] %s\n", i+1, cmd)
		session, err := client.NewSession()
		if err != nil {
			fmt.Printf("  Session error: %v\n", err)
			continue
		}
		output, err := session.CombinedOutput(cmd)
		session.Close()
		outStr := strings.TrimSpace(string(output))
		if outStr != "" {
			fmt.Println(outStr)
		}
		if err != nil {
			fmt.Printf("  (rc=%v)\n", err)
		}
	}

	fmt.Println("\n=== Download complete on 10.1.81.41 ===")
	os.Exit(0)
}
