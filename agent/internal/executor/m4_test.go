package executor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSSOutput(t *testing.T) {
	procNetTCP := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 ffff880000000000 100 0 0 10 0
   1: 00000000:0CEA 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 67890 1 ffff880000000000 100 0 0 10 0
   2: 00000000:8124 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 11111 1 ffff880000000000 100 0 0 10 0
`
	ports := parseProcNetTCP(procNetTCP)
	assert.Contains(t, ports, 22)     // 0x0016 = 22
	assert.Contains(t, ports, 3306)   // 0x0CEA = 3306
	assert.Contains(t, ports, 33060)  // 0x8124 = 33060
}

func TestParseProcNetTCP_Empty(t *testing.T) {
	ports := parseProcNetTCP("")
	assert.Empty(t, ports)
}

func TestScanUsedPorts_M4(t *testing.T) {
	scanner := NewPortScanner()
	ports, err := scanner.ScanUsedPorts()
	if err != nil {
		t.Skip("port scanning not available on this OS")
	}
	t.Logf("Used ports: %v", ports)
	assert.NotNil(t, ports)
}

func TestScanMySQLDataDirs_M4(t *testing.T) {
	scanner := NewPortScanner()

	ports, err := scanner.ScanMySQLDataDirs("/nonexistent/base/path")
	assert.NoError(t, err)
	assert.Empty(t, ports)
}

func TestFindAvailablePort_SkipsReserved(t *testing.T) {
	scanner := NewPortScanner()

	port, err := scanner.FindAvailablePort(33061, nil)
	require.NoError(t, err)
	assert.NotEqual(t, 33061, port, "Should skip MGR port")

	port, err = scanner.FindAvailablePort(4567, nil)
	require.NoError(t, err)
	assert.NotEqual(t, 4567, port, "Should skip PXC port")
}

func TestFindAvailablePort_SkipsExcluded(t *testing.T) {
	scanner := NewPortScanner()

	exclude := []int{3306, 3307, 3308}
	port, err := scanner.FindAvailablePort(3306, exclude)
	require.NoError(t, err)
	for _, ex := range exclude {
		assert.NotEqual(t, ex, port)
	}
}

func TestFindAvailablePort_InvalidRange(t *testing.T) {
	scanner := NewPortScanner()

	_, err := scanner.FindAvailablePort(0, nil)
	assert.Error(t, err)

	_, err = scanner.FindAvailablePort(65536, nil)
	assert.Error(t, err)
}

func TestMySQLReservedPortFor_M4(t *testing.T) {
	assert.Equal(t, "mgr", MySQLReservedPortFor(33061))
	assert.Equal(t, "pxc", MySQLReservedPortFor(4567))
	assert.Equal(t, "pxc", MySQLReservedPortFor(4568))
	assert.Equal(t, "pxc", MySQLReservedPortFor(4444))
	assert.Equal(t, "", MySQLReservedPortFor(3306))
	assert.Equal(t, "", MySQLReservedPortFor(22))
}

func TestIsMySQLReservedPort_M4(t *testing.T) {
	assert.True(t, IsMySQLReservedPort(33061))
	assert.True(t, IsMySQLReservedPort(4567))
	assert.False(t, IsMySQLReservedPort(3306))
	assert.False(t, IsMySQLReservedPort(3307))
}

func TestExcludeMySQLReservedPorts_M4(t *testing.T) {
	input := []int{3306, 33061, 3307, 4567, 4568, 4444, 3308}
	result := ExcludeMySQLReservedPorts(input)

	for _, p := range result {
		assert.False(t, IsMySQLReservedPort(p), "port %d should be excluded", p)
	}
	assert.Contains(t, result, 3306)
	assert.Contains(t, result, 3307)
	assert.Contains(t, result, 3308)
}

func TestFindAvailableDataDir_M4(t *testing.T) {
	scanner := NewPortScanner()

	path := scanner.FindAvailableDataDir("/data", 3306)
	assert.Equal(t, filepath.Join("/data", "mysql_3306"), path)

	path = scanner.FindAvailableDataDir("/data/mysql", 33061)
	assert.Equal(t, filepath.Join("/data/mysql", "mysql_33061"), path)

	path = scanner.FindAvailableDataDir("", 3306)
	assert.Equal(t, filepath.Join("/data", "mysql_3306"), path, "empty basePath should default to /data")
}

func TestNewPortScanner_M4(t *testing.T) {
	scanner := NewPortScanner()
	assert.NotNil(t, scanner)
}

func TestCgroupManager_GenerateSystemdOverride(t *testing.T) {
	m := NewCgroupManager()

	dir, file, err := m.GenerateSystemdOverride(CgroupConfig{
		Port:          3306,
		MemoryLimitMB: 2048,
		CPUQuotaPct:   200,
	})
	require.NoError(t, err)
	assert.Contains(t, dir, "mysqld_3306.service.d")
	assert.Contains(t, file, "limits.conf")
}

func TestCgroupManager_GenerateOverrideContent(t *testing.T) {
	m := NewCgroupManager()

	content := m.GenerateOverrideContent(3306, 4096, 300)
	assert.Contains(t, content, "MemoryMax=4096M")
	assert.Contains(t, content, "CPUQuota=300%")
	assert.Contains(t, content, "[Service]")
}

func TestCgroupManager_Defaults(t *testing.T) {
	m := NewCgroupManager()
	assert.Equal(t, 70, m.DefaultMemoryPercent)
	assert.Equal(t, 100, m.DefaultCPUMultiplier)
}

func TestCgroupManager_GetDefaultMemoryLimit(t *testing.T) {
	m := NewCgroupManager()
	assert.Equal(t, 70, m.DefaultMemoryPercent)
}

func TestCgroupManager_GetDefaultCPUQuota(t *testing.T) {
	m := NewCgroupManager()
	assert.Equal(t, 100, m.DefaultCPUMultiplier)
}
