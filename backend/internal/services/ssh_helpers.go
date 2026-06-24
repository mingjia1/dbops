package services

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"golang.org/x/crypto/ssh"
)

// NewSSHClient creates an SSH client connection to a host.
func NewSSHClient(address string, port int, user, password string) (*ssh.Client, error) {
	auth := []ssh.AuthMethod{ssh.Password(password)}
	if signer, err := ssh.ParsePrivateKey([]byte(password)); err == nil {
		auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(address, strconv.Itoa(port)), config)
}

// RunSSH executes a command over SSH and returns stdout+stderr.
func RunSSH(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(command)
	return string(out), err
}

// SCPDownload downloads a file from a remote host to a local path via SCP.
func SCPDownload(client *ssh.Client, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()
	session.Stdout = localFile
	cmd := fmt.Sprintf("cat %s", remotePath)
	return session.Run(cmd)
}

// sshClientForHost creates an SSH client from a Host model.
func sshClientForHost(host *models.Host, credential string) (*ssh.Client, error) {
	return NewSSHClient(host.Address, host.SSHPort, host.SSHUser, credential)
}
