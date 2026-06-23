package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigWriter_WriteMySQLConfig(t *testing.T) {
	w := NewConfigWriter()
	path, err := w.WriteMySQLConfig(context.Background(), 3306, "[mysqld]\nport=3306\n", t.TempDir())
	require.NoError(t, err)
	assert.Contains(t, path, "mysqld_3306.cnf")
}

func TestConfigWriter_WriteMySQLConfig_EmptyContent(t *testing.T) {
	w := NewConfigWriter()
	_, err := w.WriteMySQLConfig(context.Background(), 3306, "", "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestConfigWriter_WriteMySQLConfig_ZeroPort(t *testing.T) {
	w := NewConfigWriter()
	_, err := w.WriteMySQLConfig(context.Background(), 0, "[mysqld]", "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestConfigWriter_WriteMHAConfig(t *testing.T) {
	w := NewConfigWriter()
	path, err := w.WriteMHAConfig(context.Background(), "cluster-001", "[server default]\nuser=root\n")
	require.NoError(t, err)
	assert.Contains(t, path, "cluster-001.cnf")
}

func TestConfigWriter_WriteMHAConfig_EmptyContent(t *testing.T) {
	w := NewConfigWriter()
	_, err := w.WriteMHAConfig(context.Background(), "cluster-001", "")
	assert.Error(t, err)
}

func TestUpgradeModeConstants(t *testing.T) {
	assert.Equal(t, "in_place", UpgradeModeInPlace)
	assert.Equal(t, "logical", UpgradeModeLogical)
	assert.Equal(t, "rolling", UpgradeModeRolling)
	assert.Equal(t, "rolling_arch", UpgradeModeRollingArch)
}
