package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionAdapter_MySQL57(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mysql", "5.7.44")

	assert.Equal(t, "mysql", hints.Flavor)
	assert.Equal(t, "5.7", hints.MajorMinor)
	assert.Equal(t, "mysqld", hints.BinaryName)
	assert.True(t, hints.GTIDMode)
	assert.False(t, hints.GTIDDomainID)
	assert.Equal(t, "", hints.AuthPlugin)
	assert.True(t, hints.MysqlUpgrade)
}

func TestVersionAdapter_MySQL80(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mysql", "8.0.36")

	assert.Equal(t, "8.0", hints.MajorMinor)
	assert.True(t, hints.GTIDMode)
	assert.Equal(t, "", hints.AuthPlugin)
	assert.False(t, hints.MysqlUpgrade)
}

func TestVersionAdapter_MySQL84(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mysql", "8.4.0")

	assert.Equal(t, "8.4", hints.MajorMinor)
	assert.Equal(t, "", hints.AuthPlugin)
	assert.Contains(t, hints.Notes, "LTS")
}

func TestVersionAdapter_MariaDB1011(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mariadb", "10.11.4")

	assert.Equal(t, "mariadbd", hints.BinaryName)
	assert.False(t, hints.GTIDMode)
	assert.True(t, hints.GTIDDomainID)
	assert.Equal(t, "", hints.AuthPlugin)
	assert.False(t, hints.MysqlUpgrade)
	assert.Contains(t, hints.Notes, "mariadbd")
}

func TestVersionAdapter_MariaDB105(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mariadb", "10.5.20")

	assert.Equal(t, "mariadbd", hints.BinaryName)
	assert.Contains(t, hints.Notes, "mariadbd")
}

func TestVersionAdapter_Percona80(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("percona", "8.0.36-28")

	assert.Equal(t, "mysqld", hints.BinaryName)
	assert.True(t, hints.GTIDMode)
	assert.Equal(t, "", hints.AuthPlugin)
	assert.False(t, hints.MysqlUpgrade)
}

func TestVersionAdapter_GetDockerImage(t *testing.T) {
	a := NewVersionAdapter()

	assert.Equal(t, "mysql:8.0.36", a.GetDockerImage("mysql", "8.0.36"))
	assert.Equal(t, "mariadb:10.11.4", a.GetDockerImage("mariadb", "10.11.4"))
	assert.Equal(t, "percona/percona-server:8.0.36-28", a.GetDockerImage("percona", "8.0.36-28"))
}

func TestVersionAdapter_GetServiceName(t *testing.T) {
	a := NewVersionAdapter()

	assert.Equal(t, "mysqld_3306", a.GetServiceName("mysql", 3306))
	assert.Equal(t, "mariadbd_3307", a.GetServiceName("mariadb", 3307))
	assert.Equal(t, "mysqld_3306", a.GetServiceName("percona", 3306))
}

func TestVersionAdapter_InnodbDefaults(t *testing.T) {
	a := NewVersionAdapter()
	hints := a.GetConfigHints("mysql", "8.0.36")
	assert.Equal(t, "128M", hints.InnodbDefaults["innodb_buffer_pool_chunk_size"])
}

func TestMajorMinor_M5(t *testing.T) {
	assert.Equal(t, "8.0", majorMinor("8.0.36"))
	assert.Equal(t, "5.7", majorMinor("5.7.44"))
	assert.Equal(t, "10.11", majorMinor("10.11.4"))
	assert.Equal(t, "8", majorMinor("8"))
}
