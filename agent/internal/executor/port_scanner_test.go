package executor

import (
	"strings"
	"testing"
)

func TestMySQLReservedPorts(t *testing.T) {
	tests := []struct {
		name  string
		port  int
		owned string
	}{
		{"MGR local port", 33061, "mgr"},
		{"PXC cluster port", 4567, "pxc"},
		{"PXC IST port", 4568, "pxc"},
		{"PXC SST port", 4444, "pxc"},
		{"Standard MySQL", 3306, ""},
		{"MySQL alternative", 3307, ""},
		{"MHA manager", 22, ""}, // MHA uses SSH, no dedicated port
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MySQLReservedPortFor(tt.port)
			if got != tt.owned {
				t.Errorf("MySQLReservedPortFor(%d) = %q, want %q", tt.port, got, tt.owned)
			}
		})
	}
}

func TestIsMySQLReservedPort(t *testing.T) {
	tests := []struct {
		port int
		want bool
	}{
		{33061, true},   // MGR
		{4567, true},    // PXC
		{4568, true},    // PXC
		{4444, true},    // PXC
		{3306, false},   // Normal MySQL port
		{3307, false},   // Normal MySQL port
		{33060, false},  // MySQL X protocol
		{22, false},     // SSH
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := IsMySQLReservedPort(tt.port); got != tt.want {
				t.Errorf("IsMySQLReservedPort(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestFindAvailablePort(t *testing.T) {
	scanner := NewPortScanner()

	// Test finding a port from a known start
	port, err := scanner.FindAvailablePort(3306, nil)
	if err != nil {
		t.Fatalf("FindAvailablePort(3306, nil) unexpected error: %v", err)
	}
	if port < 3306 || port > 65535 {
		t.Errorf("FindAvailablePort returned invalid port %d", port)
	}
	t.Logf("Found available port: %d", port)

	// Test with exclusions
	exclude := []int{3306, 3307, 3308}
	port2, err := scanner.FindAvailablePort(3306, exclude)
	if err != nil {
		t.Fatalf("FindAvailablePort(3306, exclude) unexpected error: %v", err)
	}
	if port2 < 3306 || port2 > 65535 {
		t.Errorf("FindAvailablePort returned invalid port %d", port2)
	}
	// Verify port2 is not in exclude list
	for _, excluded := range exclude {
		if port2 == excluded {
			t.Errorf("FindAvailablePort returned excluded port %d", port2)
		}
	}
	t.Logf("Found available port (with exclusions %v): %d", exclude, port2)

	// Test that reserved ports are always excluded
	port3, err := scanner.FindAvailablePort(33061, nil)
	if err != nil {
		t.Fatalf("FindAvailablePort(33061, nil) unexpected error: %v", err)
	}
	if port3 == 33061 {
		t.Errorf("FindAvailablePort should skip MySQL reserved port 33061 (MGR)")
	}
	t.Logf("FindAvailablePort(33061) correctly skipped reserved port, got: %d", port3)

	// Test upper boundary
	_, err = scanner.FindAvailablePort(65536, nil)
	if err == nil {
		t.Errorf("FindAvailablePort(65536) should error on invalid start port")
	}
}

func TestFindAvailableDataDir(t *testing.T) {
	scanner := NewPortScanner()

	// Test data dir path generation
	path := scanner.FindAvailableDataDir("/data", 3306)
	if !strings.HasSuffix(path, "mysql_3306") {
		t.Errorf("FindAvailableDataDir(\"/data\", 3306) = %q, should end with mysql_3306", path)
	}

	path2 := scanner.FindAvailableDataDir("/data/mysql", 33061)
	if !strings.HasSuffix(path2, "mysql_33061") {
		t.Errorf("FindAvailableDataDir(\"/data/mysql\", 33061) = %q, should end with mysql_33061", path2)
	}
}

func TestNewPortScanner(t *testing.T) {
	scanner := NewPortScanner()
	if scanner == nil {
		t.Fatal("NewPortScanner() returned nil")
	}
}

func TestExcludeMySQLReservedPorts(t *testing.T) {
	ports := []int{3306, 33061, 3307, 4567, 4568, 4444, 3308, 33060}
	filtered := ExcludeMySQLReservedPorts(ports)

	// Should keep normal MySQL ports but remove architecture-specific ports
	for _, p := range filtered {
		if IsMySQLReservedPort(p) {
			t.Errorf("ExcludeMySQLReservedPorts should have removed %d but it remained", p)
		}
	}

	// Verify that normal ports are kept
	hasNormalPort := false
	for _, p := range filtered {
		if p == 3306 || p == 3307 || p == 3308 {
			hasNormalPort = true
			break
		}
	}
	if !hasNormalPort {
		t.Error("ExcludeMySQLReservedPorts removed normal MySQL ports, should keep them")
	}

	t.Logf("Filtered ports: %v (from %v)", filtered, ports)
}
