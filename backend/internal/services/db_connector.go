package services

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoSQLConnector is returned when an engine flavor has no usable Go driver in
// this build. Callers must fall back to TCP reachability and SSH process
// discovery instead of treating this as a failure.
var ErrNoSQLConnector = errors.New("no SQL connector available for this database flavor")

// DBConnector is the minimal read-only surface the platform needs to onboard and
// monitor a database instance. It deliberately excludes any write or
// administrative operation: engines behind this interface are managed under
// tiered onboarding and their MySQL-specific operations are refused by the
// FlavorCapability gate.
type DBConnector interface {
	// Flavor returns the normalized engine flavor this connector speaks.
	Flavor() string
	// Ping verifies the instance accepts connections and answers queries.
	Ping(ctx context.Context) error
	// ServerVersion returns the engine's self-reported version string.
	ServerVersion(ctx context.Context) (string, error)
	// Close releases the underlying connection pool.
	Close() error
}

// ConnectorTarget describes where and how to connect. Password must already be
// decrypted by the caller; connectors never touch the encryption key.
type ConnectorTarget struct {
	Flavor   string
	Host     string
	Port     int
	Username string
	Password string
	Database string
	Timeout  time.Duration
}

const defaultConnectorTimeout = 5 * time.Second

func (t ConnectorTarget) timeout() time.Duration {
	if t.Timeout <= 0 {
		return defaultConnectorTimeout
	}
	return t.Timeout
}

// NewConnector returns a read-only connector for the target's engine flavor.
//
// Flavors are grouped by wire protocol, not by vendor:
//   - MySQL protocol: mysql, mariadb, percona, oceanbase, and the MySQL-compatible
//     cloud variants (gaussdb-mysql, polardb-mysql, tdsql-mysql)
//   - PostgreSQL protocol: kingbase, opengauss, highgo, gbase8a, shentong
//   - dm and gbase8s have no usable pure-Go driver and return ErrNoSQLConnector
//
// Unknown flavors fall back to the MySQL connector, matching HasCapability's
// treatment of unknown flavors as MySQL-compatible.
func NewConnector(t ConnectorTarget) (DBConnector, error) {
	if t.Host == "" {
		return nil, fmt.Errorf("connector target host is required")
	}
	if t.Port <= 0 {
		return nil, fmt.Errorf("connector target port is required")
	}

	flavor := normalizeFlavor(t.Flavor)
	t.Flavor = flavor

	switch flavor {
	case "mysql", "mariadb", "percona", "oceanbase", "gaussdb-mysql", "polardb-mysql", "tdsql-mysql":
		return newMySQLConnector(t)
	case "kingbase", "opengauss", "highgo", "gbase8a", "shentong":
		return newPostgresConnector(t)
	case "dm":
		// Dameng DM speaks a proprietary protocol: neither MySQL nor PostgreSQL
		// compatible, with no vendor-supported pure-Go driver. Health checks
		// degrade to TCP reachability plus SSH process discovery.
		return nil, fmt.Errorf("%w: %s speaks a proprietary protocol with no pure-Go driver", ErrNoSQLConnector, flavor)
	case "gbase8s":
		// Informix protocol: no pure-Go driver exists. Same degradation as dm.
		return nil, fmt.Errorf("%w: %s speaks the Informix protocol", ErrNoSQLConnector, flavor)
	default:
		return newMySQLConnector(t)
	}
}
