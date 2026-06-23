package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHAInstaller_InstallReplicationSetup(t *testing.T) {
	h := &HAInstaller{SkipOSUserCheck: true}
	result, err := h.InstallReplicationSetup(context.Background(), HAInstallRequest{
		Flavor: "mysql", Port: 3306, DataDir: t.TempDir(), Basedir: "/opt/mysql",
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.NotEmpty(t, result.Steps)
	assert.Contains(t, result.ConfigPath, "mysqld_3306.cnf")
}

func TestHAInstaller_InstallReplicationSetup_MariaDB(t *testing.T) {
	h := &HAInstaller{SkipOSUserCheck: true}
	result, err := h.InstallReplicationSetup(context.Background(), HAInstallRequest{
		Flavor: "mariadb", Port: 3307, DataDir: t.TempDir(), Basedir: "/opt/mariadb",
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.BinaryPath, "mariadbd")
}

func TestHAInstaller_InstallMHA(t *testing.T) {
	h := NewHAInstaller()
	result, err := h.InstallMHA(context.Background(), HAInstallRequest{ArchType: "mha", Port: 3306, DataDir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.ConfigPath, "mha")
}

func TestHAInstaller_InstallMGR(t *testing.T) {
	h := &HAInstaller{SkipOSUserCheck: true}
	result, err := h.InstallMGR(context.Background(), HAInstallRequest{Flavor: "mysql", Port: 3306, DataDir: t.TempDir(), Basedir: "/opt/mysql"})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.ConfigPath, "mysqld_3306.cnf")
}

func TestHAInstaller_InstallPXC(t *testing.T) {
	h := NewHAInstaller()
	result, err := h.InstallPXC(context.Background(), HAInstallRequest{Port: 3306, DataDir: t.TempDir(), Basedir: "/opt/percona"})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.ConfigPath, "mysqld_3306.cnf")
}

func TestGenerateReplicaCnf_MySQL(t *testing.T) {
	cnf := generateReplicaCnf("mysql", 3306, "/data/mysql_3306", "/opt/mysql")
	assert.Contains(t, cnf, "gtid_mode=ON")
	assert.Contains(t, cnf, "server-id=3306")
	assert.NotContains(t, cnf, "gtid_domain_id")
}

func TestGenerateReplicaCnf_MariaDB(t *testing.T) {
	cnf := generateReplicaCnf("mariadb", 3307, "/data/mysql_3307", "/opt/mariadb")
	assert.Contains(t, cnf, "gtid_domain_id=3307")
	assert.NotContains(t, cnf, "gtid_mode")
}

func TestGenerateMGRCnf(t *testing.T) {
	cnf := generateMGRCnf("mysql", 3306, "/data", "/opt/mysql")
	assert.Contains(t, cnf, "group_replication")
	assert.Contains(t, cnf, "group_replication_start_on_boot=OFF")
}

func TestGeneratePXCCnf(t *testing.T) {
	cnf := generatePXCCnf(3306, "/data", "/opt/percona")
	assert.Contains(t, cnf, "wsrep_on=ON")
	assert.Contains(t, cnf, "wsrep_sst_method=xtrabackup-v2")
}

func TestGenerateMHACnf(t *testing.T) {
	cnf := generateMHACnf("mha", 3306)
	assert.Contains(t, cnf, "[server default]")
	assert.Contains(t, cnf, "repl_user=repl")
	assert.Contains(t, cnf, "[server1]")
}

func TestNewHAInstaller(t *testing.T) {
	h := NewHAInstaller()
	assert.NotNil(t, h)
}
