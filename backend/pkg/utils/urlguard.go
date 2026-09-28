package utils

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoinUnder P0-3: 把用户提供的子路径安全地拼到 baseDir 下, 防止路径穿越.
// 返回清洗后的绝对路径. 任何试图逃出 baseDir 的输入 (".."、绝对路径、盘符等) 都返回错误.
func SafeJoinUnder(baseDir, subPath string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	cleaned := filepath.Clean(subPath)
	// 显式拒绝: 绝对路径 / 带根斜杠 / 盘符 / 向上跳级. Windows 下 filepath.IsAbs 不认 "/x",
	// 但 Linux 上 Join(base, "/x") 会直接逃出 base, 因此按平台无关规则拦截.
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") ||
		filepath.VolumeName(cleaned) != "" ||
		cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid sub path %q: escapes base dir", subPath)
	}
	joined := filepath.Join(baseAbs, cleaned)
	rel, err := filepath.Rel(baseAbs, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid sub path %q: escapes base dir", subPath)
	}
	return joined, nil
}

// ValidateExternalURL P0-3: SSRF 防护. 仅允许 http/https, 且拒绝解析到私网/环回/链路本地的主机名.
// 内网镜像站场景可通过 DBOPS_SSRF_ALLOW_PRIVATE_HOSTS=1 显式放行 (离线内网部署).
func ValidateExternalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("only http/https urls are allowed, got scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("url has no host")
	}
	allowPrivate := os.Getenv("DBOPS_SSRF_ALLOW_PRIVATE_HOSTS") == "1"
	if !allowPrivate && isPrivateOrLocalHost(host) {
		return "", fmt.Errorf("host %q resolves to a private/loopback address; set DBOPS_SSRF_ALLOW_PRIVATE_HOSTS=1 to allow internal mirrors", host)
	}
	return u.String(), nil
}

func isPrivateOrLocalHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	// IP 字面量无需 DNS 解析.
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateOrLocalIP(ip)
	}
	// 域名: 解析后逐个地址检查.
	ips, err := net.LookupIP(host)
	if err != nil {
		// 解析失败视为可疑, 拒绝.
		return true
	}
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return true
		}
	}
	return false
}

var privateV4Ranges = []net.IPNet{
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("169.254.0.0/16"), // link-local (含云厂商 metadata 169.254.169.254)
	mustCIDR("100.64.0.0/10"),  // CGNAT / k8s pod 常见
	mustCIDR("0.0.0.0/8"),      // unspecified
}

var privateV6Ranges = []net.IPNet{
	mustCIDR("::1/128"),     // loopback
	mustCIDR("::/128"),      // unspecified
	mustCIDR("fe80::/10"),   // link-local
	mustCIDR("fc00::/7"),    // unique local
}

func mustCIDR(s string) net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("invalid cidr in urlguard: " + s)
	}
	return *n
}

func isPrivateOrLocalIP(ip net.IP) bool {
	// IPv4-mapped IPv6 先还原成 v4 再判断.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		for _, r := range privateV4Ranges {
			if r.Contains(ip) {
				return true
			}
		}
		return false
	}
	for _, r := range privateV6Ranges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
