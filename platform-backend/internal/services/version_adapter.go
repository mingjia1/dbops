package services

import (
	"fmt"
	"strings"
)

type VersionAdapter struct{}

func NewVersionAdapter() *VersionAdapter {
	return &VersionAdapter{}
}

type VersionConfigHints struct {
	Flavor         string
	MajorMinor     string
	BinaryName     string
	GTIDMode       bool
	GTIDDomainID   bool
	AuthPlugin     string
	MysqlUpgrade   bool
	InnodbDefaults map[string]string
	Notes          string
}

func (a *VersionAdapter) GetConfigHints(flavor, version string) *VersionConfigHints {
	mm := majorMinor(version)
	hints := &VersionConfigHints{
		Flavor:         flavor,
		MajorMinor:     mm,
		InnodbDefaults: make(map[string]string),
	}

	switch flavor {
	case "mariadb":
		hints.BinaryName = "mariadbd"
		hints.GTIDMode = false
		hints.GTIDDomainID = true
		hints.AuthPlugin = ""
		hints.MysqlUpgrade = false
		hints.InnodbDefaults["innodb_buffer_pool_chunk_size"] = "128M"
		if strings.HasPrefix(mm, "10.5") || strings.HasPrefix(mm, "10.6") || strings.HasPrefix(mm, "10.11") {
			hints.Notes = "MariaDB 10.5+ uses mariadbd instead of mysqld"
		}
	case "percona":
		hints.BinaryName = "mysqld"
		hints.GTIDMode = true
		hints.GTIDDomainID = false
		hints.AuthPlugin = "mysql_native_password"
		hints.MysqlUpgrade = a.needsMysqlUpgrade(mm)
		hints.InnodbDefaults["innodb_buffer_pool_chunk_size"] = "128M"
	default:
		hints.BinaryName = "mysqld"
		hints.GTIDMode = true
		hints.GTIDDomainID = false
		hints.MysqlUpgrade = a.needsMysqlUpgrade(mm)
		if mm == "5.7" {
			hints.AuthPlugin = ""
		} else {
			hints.AuthPlugin = "mysql_native_password"
		}
		hints.InnodbDefaults["innodb_buffer_pool_chunk_size"] = "128M"
		if mm == "8.4" {
			hints.Notes = "MySQL 8.4 is LTS until 2032"
		}
	}

	return hints
}

func (a *VersionAdapter) needsMysqlUpgrade(majorMinor string) bool {
	switch majorMinor {
	case "5.6", "5.7":
		return true
	case "8.0":
		return false
	default:
		return false
	}
}

func (a *VersionAdapter) GetDockerImage(flavor, version string) string {
	switch flavor {
	case "mariadb":
		return fmt.Sprintf("mariadb:%s", version)
	case "percona":
		return fmt.Sprintf("percona/percona-server:%s", version)
	default:
		return fmt.Sprintf("mysql:%s", version)
	}
}

func (a *VersionAdapter) GetServiceName(flavor string, port int) string {
	switch flavor {
	case "mariadb":
		return fmt.Sprintf("mariadbd_%d", port)
	case "percona":
		return fmt.Sprintf("mysqld_%d", port)
	default:
		return fmt.Sprintf("mysqld_%d", port)
	}
}
