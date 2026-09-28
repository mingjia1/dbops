package services

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsPath 返回 TOFU 主机密钥库的统一路径.
// 与 InstanceService 既有约定保持一致 (./data), 避免同一台主机在
// 不同服务里各存一份 known_hosts 导致互相不认识.
func knownHostsPath() string {
	return filepath.Join("./data", "known_hosts")
}

// tofuHostKeyCallback 返回 TOFU (Trust On First Use) 主机密钥回调:
//   - 主机密钥已知且匹配     -> 放行
//   - 主机密钥已知但不匹配   -> 拒绝 (疑似 MITM, 需人工处理 known_hosts)
//   - 首次连接 (未知主机)    -> 记录密钥到 known_hosts 后放行
//
// 注意必须用 errors.As 区分 knownhosts.KeyError: known_hosts 文件存在但
// 不含该主机时, knownhosts.New 的回调也返回 KeyError (Want 为空) —— 这属于
// "首次连接", 不能当成 "密钥变更" 拒绝, 否则新主机永远接不进来.
func tofuHostKeyCallback(knownHostsFile, hostAddr string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if _, statErr := os.Stat(knownHostsFile); statErr == nil {
			cb, cbErr := knownhosts.New(knownHostsFile)
			if cbErr == nil {
				verifyErr := cb(hostname, remote, key)
				if verifyErr == nil {
					return nil
				}
				var keyErr *knownhosts.KeyError
				if errors.As(verifyErr, &keyErr) && len(keyErr.Want) == 0 {
					// 未知主机 -> 走首次记录
				} else {
					return fmt.Errorf("host key verification failed for %s: host key has changed! Possible MITM attack; if the host was legitimately reprovisioned, remove its line from %s and retry", hostAddr, knownHostsFile)
				}
			}
			// knownhosts.New 解析失败 (文件损坏等) -> 也走首次记录兜底
		}
		log.Printf("WARN: First SSH connection to %s — recording host key to %s", hostAddr, knownHostsFile)
		if err := os.MkdirAll(filepath.Dir(knownHostsFile), 0o755); err != nil {
			return fmt.Errorf("cannot create known_hosts dir: %w", err)
		}
		f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("cannot write known_hosts: %w", err)
		}
		defer f.Close()
		line := knownhosts.Line([]string{knownhosts.Normalize(hostAddr)}, key)
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write known_hosts: %w", err)
		}
		return nil
	}
}

// insecureHostKeyAllowed 允许在明确设置 DBOPS_SSH_INSECURE=1 时跳过主机密钥校验.
// 仅用于无法预知密钥的临时环境, 生产环境严禁开启 (对应巡检报告 P0-4).
func insecureHostKeyAllowed() bool {
	return os.Getenv("DBOPS_SSH_INSECURE") == "1"
}

// NewSSHClient creates an SSH client connection to a host.
// P0-4: 主机密钥验证由 InsecureIgnoreHostKey 改为 TOFU 回调.
func NewSSHClient(address string, port int, user, password string) (*ssh.Client, error) {
	auth := []ssh.AuthMethod{ssh.Password(password)}
	if signer, err := ssh.ParsePrivateKey([]byte(password)); err == nil {
		auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	hostKeyCallback := tofuHostKeyCallback(knownHostsPath(), address)
	if insecureHostKeyAllowed() {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
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
