package executor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRelayDownloader_TarballName(t *testing.T) {
	d := NewRelayDownloader()

	assert.Equal(t, "mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz", d.tarballName("mysql", "8.0.36"))
	assert.Equal(t, "mariadb-10.11.4-linux-x86_64.tar.gz", d.tarballName("mariadb", "10.11.4"))
	assert.Equal(t, "Percona-Server-8.0.36-28-Linux.x86_64.glibc2.17.tar.gz", d.tarballName("percona", "8.0.36-28"))
}

func TestRelayDownloader_BinaryPath(t *testing.T) {
	d := NewRelayDownloader()

	assert.Equal(t, filepath.Join("/opt/mysql-8.0.36", "bin", "mysqld"), d.BinaryPath("/opt/mysql-8.0.36", "mysql"))
	assert.Equal(t, filepath.Join("/opt/mariadb-10.11.4", "bin", "mariadbd"), d.BinaryPath("/opt/mariadb-10.11.4", "mariadb"))
	assert.Equal(t, filepath.Join("/opt/percona", "bin", "mysqld"), d.BinaryPath("/opt/percona", "percona"))
}

func TestRelayDownloader_MissingURL(t *testing.T) {
	d := NewRelayDownloader()
	_, err := d.DownloadFromRelay(nil, RelayDownloadConfig{
		Version: "8.0.36",
		Basedir: "/tmp/test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "relay URL")
}

func TestRelayDownloader_MissingVersion(t *testing.T) {
	d := NewRelayDownloader()
	_, err := d.DownloadFromRelay(nil, RelayDownloadConfig{
		RelayURL: "http://relay.example.com",
		Basedir:  "/tmp/test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestExtractMajorMinor_Agent(t *testing.T) {
	assert.Equal(t, "8.0", extractMajorMinor("8.0.36"))
	assert.Equal(t, "10.11", extractMajorMinor("10.11.4"))
}

func TestAPTSourceManager_Config(t *testing.T) {
	m := NewAPTSourceManager()
	assert.NotNil(t, m)
}
