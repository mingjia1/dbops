package executor

import "testing"

// TestNeedsMysqlUpgrade A4: mysql_upgrade 在 MySQL 8.0.17+ 被移除.
// 跳过该工具是必要修复, 否则升 8.0.17+ 必失败.
func TestNeedsMysqlUpgrade(t *testing.T) {
	cases := []struct {
		version  string
		expected bool
	}{
		// 8.0.17+ 跳过 (server 启动自动处理)
		{"8.0.17", false},
		{"8.0.36", false},
		{"8.1.0", false},
		{"8.1.5", false},
		{"9.0.0", false},
		// 8.0.0 ~ 8.0.16 仍需要
		{"8.0.16", true},
		{"8.0.0", true},
		{"8.0", true},
		{"7.4.0", true},
		{"5.7.44", true},
		{"5.6.51", true},
		// 解析失败保守执行
		{"", true},
		{"unknown", true},
		// "8" 单独
		{"8", true},
	}

	for _, c := range cases {
		got := needsMysqlUpgrade(c.version)
		if got != c.expected {
			t.Errorf("needsMysqlUpgrade(%q) = %v, want %v", c.version, got, c.expected)
		}
	}
}

// TestParseVersion 辅助 parseVersion 行为.
func TestParseVersion(t *testing.T) {
	cases := []struct {
		in                      string
		major, minor, patch     int
		ok                      bool
	}{
		{"8.0.36", 8, 0, 36, true},
		{"8.0.17", 8, 0, 17, true},
		{"8.0.16", 8, 0, 16, true},
		{"8.0", 8, 0, 0, true},
		{"8.1.0", 8, 1, 0, true},
		{"5.7.44", 5, 7, 44, true},
		{"10.11.5", 10, 11, 5, true},
		{"8", 8, 0, 0, true},
		{"", 0, 0, 0, false},
		{"junk", 0, 0, 0, false},
	}
	for _, c := range cases {
		maj, min, pat, ok := parseVersion(c.in)
		if ok != c.ok || maj != c.major || min != c.minor || pat != c.patch {
			t.Errorf("parseVersion(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
				c.in, maj, min, pat, ok, c.major, c.minor, c.patch, c.ok)
		}
	}
}
