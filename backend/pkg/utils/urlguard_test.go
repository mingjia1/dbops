package utils

import (
	"strings"
	"testing"
)

func TestSafeJoinUnderRejectsTraversal(t *testing.T) {
	cases := []string{
		"../../etc",
		"..\\..\\windows",
		"/etc/passwd",
		"C:\\Windows",
		"mysql/../../../boot",
		"..",
	}
	for _, sub := range cases {
		if _, err := SafeJoinUnder(t.TempDir(), sub); err == nil {
			t.Errorf("expected rejection for sub path %q", sub)
		}
	}
}

func TestSafeJoinUnderAcceptsNormalSubPath(t *testing.T) {
	base := t.TempDir()
	joined, err := SafeJoinUnder(base, "mysql/8.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(joined, base) {
		t.Fatalf("joined path %q not under base %q", joined, base)
	}
	// 空子路径允许 (直接落 base).
	if _, err := SafeJoinUnder(base, ""); err != nil {
		t.Fatalf("empty sub path should be allowed: %v", err)
	}
	// 单个 "." 也应落 base 本身.
	joined2, err := SafeJoinUnder(base, ".")
	if err != nil || joined2 != base {
		t.Fatalf("dot sub path should resolve to base, got %q err=%v", joined2, err)
	}
}

func TestValidateExternalURLRejectsBadScheme(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://x", "gopher://y", "http://"} {
		if _, err := ValidateExternalURL(raw); err == nil {
			t.Errorf("expected rejection for %q", raw)
		}
	}
}

func TestValidateExternalURLRejectsLoopbackIP(t *testing.T) {
	if _, err := ValidateExternalURL("http://127.0.0.1:8080/x"); err == nil {
		t.Fatal("expected loopback rejection")
	}
	if _, err := ValidateExternalURL("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("expected link-local (metadata service) rejection")
	}
	if _, err := ValidateExternalURL("http://10.0.0.5/x"); err == nil {
		t.Fatal("expected private-range rejection")
	}
}

func TestValidateExternalURLAllowsPublicIPAndDomain(t *testing.T) {
	// 用公网 IP 字面量做确定性断言 (不依赖测试环境能否解析外部域名).
	if _, err := ValidateExternalURL("https://93.184.216.34/downloads/mysql.tar.xz"); err != nil {
		t.Fatalf("public ip url should pass: %v", err)
	}
	// localhost 字面量无条件拒绝.
	if _, err := ValidateExternalURL("http://localhost:8080/x"); err == nil {
		t.Fatal("expected localhost rejection")
	}
}
