package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

// mysqlConnector speaks the MySQL wire protocol.
type mysqlConnector struct {
	flavor string
	db     *sql.DB
}

// buildMySQLDSN constructs a MySQL DSN using mysql.Config.FormatDSN so that
// special characters in credentials are escaped rather than concatenated into
// the DSN string. This mirrors the construction previously inlined in
// HealthCheckService and must stay behaviourally identical.
func buildMySQLDSN(t ConnectorTarget) string {
	timeout := t.timeout()
	cfg := mysql.NewConfig()
	cfg.User = t.Username
	cfg.Passwd = t.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port))
	cfg.DBName = t.Database
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = timeout
	cfg.ReadTimeout = timeout
	cfg.WriteTimeout = timeout
	return cfg.FormatDSN()
}

func newMySQLConnector(t ConnectorTarget) (DBConnector, error) {
	db, err := sql.Open("mysql", buildMySQLDSN(t))
	if err != nil {
		return nil, fmt.Errorf("open mysql connection: %w", err)
	}
	return &mysqlConnector{flavor: normalizeFlavor(t.Flavor), db: db}, nil
}

func (c *mysqlConnector) Flavor() string { return c.flavor }

func (c *mysqlConnector) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *mysqlConnector) ServerVersion(ctx context.Context) (string, error) {
	var version string
	if err := c.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", fmt.Errorf("read mysql server version: %w", err)
	}
	return version, nil
}

func (c *mysqlConnector) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}
