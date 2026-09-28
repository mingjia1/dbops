package executor

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// agentKnownHostsPath 返回 agent 侧 known_hosts 路径.
// 可用 DBOPS_KNOWN_HOSTS 覆盖, 默认与 backend 侧约定一致 (./data/known_hosts).
func agentKnownHostsPath() string {
	if p := os.Getenv("DBOPS_KNOWN_HOSTS"); p != "" {
		return p
	}
	return filepath.Join("./data", "known_hosts")
}

// agentHostKeyCallback 返回 TOFU 主机密钥回调:
// 首次连接记录密钥, 之后密钥不匹配则拒绝 (抗 MITM).
// 与 backend/internal/services/ssh_helpers.go 的 tofuHostKeyCallback 逻辑一致
// (两个模块互相独立, 无法共享代码).
// DBOPS_SSH_INSECURE=1 可显式跳过校验 (临时环境逃生口, 生产禁用).
func agentHostKeyCallback(hostAddr string) ssh.HostKeyCallback {
	if os.Getenv("DBOPS_SSH_INSECURE") == "1" {
		return ssh.InsecureIgnoreHostKey()
	}
	knownHostsFile := agentKnownHostsPath()
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if _, statErr := os.Stat(knownHostsFile); statErr == nil {
			cb, cbErr := knownhosts.New(knownHostsFile)
			if cbErr == nil {
				verifyErr := cb(hostname, remote, key)
				if verifyErr == nil {
					return nil
				}
				var keyErr *knownhosts.KeyError
				if !(errors.As(verifyErr, &keyErr) && len(keyErr.Want) == 0) {
					return fmt.Errorf("host key verification failed for %s: host key has changed! Possible MITM attack; if the host was legitimately reprovisioned, remove its line from %s and retry", hostAddr, knownHostsFile)
				}
				// 未知主机 -> 走首次记录
			}
			// knownhosts.New 解析失败 -> 走首次记录兜底
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
