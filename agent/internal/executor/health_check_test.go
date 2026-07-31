package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMySQLAdminHealthCheckArgsHonorsTLS(t *testing.T) {
	args := mysqlAdminHealthCheckArgs("10.0.0.8", 3306, "monitor", "8.0.36", true)

	assert.Equal(t, []string{
		"mysqladmin", "ping", "-h", "10.0.0.8", "-P", "3306", "-u", "monitor",
		"--ssl-mode=REQUIRED", "--get-server-public-key",
	}, args)
}

func TestMySQLAdminHealthCheckArgsUsesConfiguredEndpointOnly(t *testing.T) {
	args := mysqlAdminHealthCheckArgs("10.0.0.9", 3307, "root", "5.7.44", false)

	assert.Equal(t, []string{"mysqladmin", "ping", "-h", "10.0.0.9", "-P", "3307", "-u", "root"}, args)
}
