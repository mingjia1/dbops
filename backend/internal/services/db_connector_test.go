package services

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeName returns the concrete type of a connector for dispatch assertions.
func typeName(v interface{}) string {
	return fmt.Sprintf("%T", v)
}

func TestNewConnectorDispatchesByFlavor(t *testing.T) {
	tests := []struct {
		flavor   string
		wantType string
	}{
		{flavor: "mysql", wantType: "*services.mysqlConnector"},
		{flavor: "mariadb", wantType: "*services.mysqlConnector"},
		{flavor: "percona", wantType: "*services.mysqlConnector"},
		{flavor: "oceanbase", wantType: "*services.mysqlConnector"},
		{flavor: "tidb", wantType: "*services.mysqlConnector"},
		{flavor: "gaussdb-mysql", wantType: "*services.mysqlConnector"},
		{flavor: "polardb-mysql", wantType: "*services.mysqlConnector"},
		{flavor: "tdsql-mysql", wantType: "*services.mysqlConnector"},
		{flavor: "kingbase", wantType: "*services.postgresConnector"},
		{flavor: "opengauss", wantType: "*services.postgresConnector"},
		{flavor: "highgo", wantType: "*services.postgresConnector"},
		{flavor: "gbase8a", wantType: "*services.postgresConnector"},
		{flavor: "shentong", wantType: "*services.postgresConnector"},
		// Empty flavor retains the legacy MySQL behavior.
		{flavor: "", wantType: "*services.mysqlConnector"},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			connector, err := NewConnector(ConnectorTarget{
				Flavor:   tt.flavor,
				Host:     "10.0.0.50",
				Port:     3306,
				Username: "root",
				Password: "secret",
			})
			require.NoError(t, err)
			require.NotNil(t, connector)
			defer connector.Close()
			assert.Equal(t, tt.wantType, typeName(connector))
			assert.Equal(t, normalizeFlavor(tt.flavor), connector.Flavor())
		})
	}
}

func TestNewConnectorReturnsErrNoSQLConnectorForDriverlessFlavors(t *testing.T) {
	// gbase8s speaks Informix; dm is behind the dm_driver build tag and is not
	// compiled by default. Both must degrade to TCP+SSH, signalled by a sentinel
	// error the caller can test for.
	for _, flavor := range []string{"gbase8s", "dm", "some-future-engine"} {
		t.Run(flavor, func(t *testing.T) {
			connector, err := NewConnector(ConnectorTarget{
				Flavor:   flavor,
				Host:     "10.0.0.51",
				Port:     5236,
				Username: "SYSDBA",
				Password: "secret",
			})
			require.Error(t, err)
			assert.Nil(t, connector)
			assert.True(t, errors.Is(err, ErrNoSQLConnector), "error must wrap ErrNoSQLConnector, got %v", err)
		})
	}
}

func TestNewConnectorValidatesTarget(t *testing.T) {
	_, err := NewConnector(ConnectorTarget{Flavor: "mysql", Port: 3306})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")

	_, err = NewConnector(ConnectorTarget{Flavor: "mysql", Host: "10.0.0.52"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port is required")
}

func TestBuildMySQLDSNEscapesCredentialsAndSetsTimeouts(t *testing.T) {
	dsn := buildMySQLDSN(ConnectorTarget{
		Flavor:   "mysql",
		Host:     "10.0.0.53",
		Port:     3307,
		Username: "root",
		Password: "p@ss:w/ord",
		Timeout:  7 * time.Second,
	})

	assert.Contains(t, dsn, "tcp(10.0.0.53:3307)")
	assert.Contains(t, dsn, "timeout=7s")
	assert.Contains(t, dsn, "readTimeout=7s")
	assert.Contains(t, dsn, "writeTimeout=7s")
	assert.Contains(t, dsn, "parseTime=true")
	// The raw password must not appear unescaped before the host separator.
	assert.Contains(t, dsn, "root:p@ss:w/ord@")
}

func TestBuildMySQLDSNAppliesDefaultTimeout(t *testing.T) {
	dsn := buildMySQLDSN(ConnectorTarget{Flavor: "mysql", Host: "10.0.0.54", Port: 3306})
	assert.Contains(t, dsn, "timeout=5s")
}

func TestBuildMySQLDSNHonorsTLS(t *testing.T) {
	dsn := buildMySQLDSN(ConnectorTarget{Flavor: "mysql", Host: "10.0.0.54", Port: 3306, SSLEnabled: true})
	assert.Contains(t, dsn, "tls=true")
}

func TestBuildPostgresDSNUsesVendorDefaultDatabase(t *testing.T) {
	tests := []struct {
		flavor       string
		wantDatabase string
	}{
		{flavor: "kingbase", wantDatabase: "/test"},
		{flavor: "opengauss", wantDatabase: "/postgres"},
		{flavor: "highgo", wantDatabase: "/highgo"},
		{flavor: "gbase8a", wantDatabase: "/gbase"},
		{flavor: "shentong", wantDatabase: "/OSRDB"},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			dsn := buildPostgresDSN(ConnectorTarget{
				Flavor:   tt.flavor,
				Host:     "10.0.0.55",
				Port:     54321,
				Username: "system",
				Password: "secret",
			})
			assert.Contains(t, dsn, tt.wantDatabase)
			assert.Contains(t, dsn, "10.0.0.55:54321")
			assert.Contains(t, dsn, "connect_timeout=5")
		})
	}
}

func TestBuildPostgresDSNHonoursExplicitDatabase(t *testing.T) {
	dsn := buildPostgresDSN(ConnectorTarget{
		Flavor:   "kingbase",
		Host:     "10.0.0.56",
		Port:     54321,
		Username: "system",
		Password: "secret",
		Database: "appdb",
	})
	assert.Contains(t, dsn, "/appdb")
	assert.NotContains(t, dsn, "/test")
}

func TestBuildPostgresDSNHonorsTLS(t *testing.T) {
	tlsDSN := buildPostgresDSN(ConnectorTarget{Flavor: "kingbase", Host: "10.0.0.56", Port: 54321, SSLEnabled: true})
	plainDSN := buildPostgresDSN(ConnectorTarget{Flavor: "kingbase", Host: "10.0.0.56", Port: 54321})
	assert.Contains(t, tlsDSN, "sslmode=require")
	assert.Contains(t, plainDSN, "sslmode=disable")
}

func TestBuildPostgresDSNEscapesSpecialCharactersInPassword(t *testing.T) {
	dsn := buildPostgresDSN(ConnectorTarget{
		Flavor:   "kingbase",
		Host:     "10.0.0.57",
		Port:     54321,
		Username: "system",
		Password: "p@ss/w:ord?x",
	})
	// Raw special characters must be percent-encoded so they cannot terminate
	// the userinfo section or inject query parameters.
	assert.NotContains(t, dsn, "p@ss/w:ord?x")
	assert.Contains(t, dsn, "10.0.0.57:54321")
}

func TestFilterHealthCheckTypesByCapability(t *testing.T) {
	requested := []string{"tcp", "mysql", "replication"}

	tests := []struct {
		flavor string
		want   []string
	}{
		// MySQL-protocol engines keep every check.
		{flavor: "mysql", want: []string{"tcp", "mysql", "replication"}},
		{flavor: "", want: []string{"tcp", "mysql", "replication"}},
		{flavor: "oceanbase", want: []string{"tcp", "mysql", "replication"}},
		// TiDB uses a MySQL connector for liveness only; replication remains gated.
		{flavor: "tidb", want: []string{"tcp", "mysql"}},
		// PostgreSQL-compatible engines keep SQL liveness but lose replication.
		{flavor: "kingbase", want: []string{"tcp", "mysql"}},
		{flavor: "gbase8a", want: []string{"tcp", "mysql"}},
		{flavor: "shentong", want: []string{"tcp", "mysql"}},
		// Driverless engines are reduced to TCP only.
		{flavor: "gbase8s", want: []string{"tcp"}},
		{flavor: "dm", want: []string{"tcp"}},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			assert.Equal(t, tt.want, filterHealthCheckTypesByCapability(tt.flavor, requested))
		})
	}
}

func TestEffectiveHealthCheckTypesDropsUnsupportedChecks(t *testing.T) {
	requested := []string{"tcp", "mysql", "replication"}

	gbase8s := &models.Instance{Version: models.InstanceVersion{Flavor: "gbase8s"}}
	assert.Equal(t, []string{"tcp"}, effectiveHealthCheckTypes(gbase8s, requested))

	kingbase := &models.Instance{Version: models.InstanceVersion{Flavor: "kingbase"}}
	assert.Equal(t, []string{"tcp", "mysql"}, effectiveHealthCheckTypes(kingbase, requested))

	tidb := &models.Instance{Version: models.InstanceVersion{Flavor: "tidb"}}
	assert.Equal(t, []string{"tcp", "mysql"}, effectiveHealthCheckTypes(tidb, requested))

	// MySQL instances and legacy instances with no recorded flavor are untouched.
	mysqlInst := &models.Instance{Version: models.InstanceVersion{Flavor: "mysql"}}
	assert.Equal(t, requested, effectiveHealthCheckTypes(mysqlInst, requested))
	legacy := &models.Instance{}
	assert.Equal(t, requested, effectiveHealthCheckTypes(legacy, requested))
}

func TestEffectiveHealthCheckTypesStillReducesMHAManagerToTCP(t *testing.T) {
	// Pre-existing behaviour must not regress.
	manager := &models.Instance{
		Topology: models.InstanceTopology{ReplicationMode: ClusterTypeMHA},
		Status:   models.InstanceStatus{Role: "manager"},
	}
	assert.Equal(t, []string{"tcp"}, effectiveHealthCheckTypes(manager, []string{"tcp", "mysql", "replication"}))
}
