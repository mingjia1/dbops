package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCgroupManager(t *testing.T) {
	mgr := NewCgroupManager()
	assert.NotNil(t, mgr)
	assert.Equal(t, 70, mgr.DefaultMemoryPercent)
	assert.Equal(t, 100, mgr.DefaultCPUMultiplier)
}

func TestGenerateSystemdOverride_Basic(t *testing.T) {
	m := NewCgroupManager()

	cfg := CgroupConfig{
		Port:          3306,
		MemoryLimitMB: 2048,
		CPUQuotaPct:   200,
	}

	dir, file, err := m.GenerateSystemdOverride(cfg)
	require.NoError(t, err)
	assert.Contains(t, dir, "mysqld_3306.service.d")
	assert.Contains(t, file, "limits.conf")

	// Verify file content
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "[Service]")
	assert.Contains(t, content, "MemoryMax=2048M")
	assert.Contains(t, content, "CPUQuota=200%")
	t.Logf("Generated override content:\n%s", content)
}

func TestGenerateSystemdOverride_CustomService(t *testing.T) {
	m := NewCgroupManager()

	cfg := CgroupConfig{
		Port:          3307,
		MemoryLimitMB: 1024,
		CPUQuotaPct:   100,
		ServiceName:   "custom-mysql",
	}

	dir, _, err := m.GenerateSystemdOverride(cfg)
	require.NoError(t, err)
	assert.Contains(t, dir, "custom-mysql.service.d")
}

func TestGenerateSystemdOverride_NoPort(t *testing.T) {
	m := NewCgroupManager()

	cfg := CgroupConfig{Port: 0}
	_, _, err := m.GenerateSystemdOverride(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestGenerateSystemdOverride_DefaultValues(t *testing.T) {
	m := NewCgroupManager()
	// Using the content-only generator
	content := m.GenerateOverrideContent(3308, 0, 0)
	assert.Contains(t, content, "MemoryMax=")
	assert.Contains(t, content, "CPUQuota=")
	assert.Contains(t, content, "[Service]")
	t.Logf("Default override content:\n%s", content)
}

func TestGenerateOverrideContent(t *testing.T) {
	m := NewCgroupManager()

	content := m.GenerateOverrideContent(3309, 4096, 300)
	assert.Contains(t, content, "MemoryMax=4096M")
	assert.Contains(t, content, "CPUQuota=300%")
	assert.Contains(t, content, "[Service]")
}

func TestWriteSystemdOverride(t *testing.T) {
	m := NewCgroupManager()

	// Uses old-style API to test compatibility
	file, err := m.WriteSystemdOverride(3310, 2048, 200)
	require.NoError(t, err)
	assert.Contains(t, file, "limits.conf")

	// Cleanup
	overrideDir := filepath.Join("/etc/systemd/system", "mysqld_3310.service.d")
	defer os.RemoveAll(overrideDir)
}

func TestReloadSystemd_NotLinux(t *testing.T) {
	m := NewCgroupManager()

	// Should fail if not on Linux
	err := m.ReloadSystemd()
	if err != nil {
		assert.Contains(t, err.Error(), "only supported on Linux")
	}
}

func TestCalculateMemoryLimit(t *testing.T) {
	assert.Equal(t, 700, CalculateMemoryLimit(1000, 1))
	assert.Equal(t, 350, CalculateMemoryLimit(1000, 2))
	assert.Equal(t, 256, CalculateMemoryLimit(100, 10), "should enforce minimum of 256MB")
	assert.Equal(t, 700, CalculateMemoryLimit(1000, 0), "instanceCount=0 defaults to 1")
}

func TestCalculateCPUQuota(t *testing.T) {
	assert.Equal(t, 200, CalculateCPUQuota(2))
	assert.Equal(t, 800, CalculateCPUQuota(8))
	assert.Equal(t, 100, CalculateCPUQuota(1))
}

func TestGenerateSystemdOverride_ZeroValues(t *testing.T) {
	m := NewCgroupManager()

	// Test with zero memory and CPU - should use defaults
	cfg := CgroupConfig{
		Port: 3311,
	}
	dir, file, err := m.GenerateSystemdOverride(cfg)
	require.NoError(t, err)
	assert.Contains(t, dir, "mysqld_3311.service.d")

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "MemoryMax=")
	assert.Contains(t, content, "CPUQuota=")
}

func TestServiceNameGeneration(t *testing.T) {
	m := NewCgroupManager()

	// With port only
	svcName := m.serviceName(CgroupConfig{Port: 3306})
	assert.Equal(t, "mysqld_3306", svcName)

	// With custom service name
	svcName = m.serviceName(CgroupConfig{Port: 3306, ServiceName: "my-mysql"})
	assert.Equal(t, "my-mysql", svcName)
}
