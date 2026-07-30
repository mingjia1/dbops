package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// postgresConnector speaks the PostgreSQL wire protocol. Kingbase ES, openGauss,
// HighGo, GBase 8a and ShenTong (OSCAR) are all PostgreSQL-compatible, so a
// single connector serves all of them.
type postgresConnector struct {
	flavor string
	db     *sql.DB
}

// defaultPostgresDatabase returns the database to connect to when the caller did
// not specify one. Each vendor ships a different bootstrap database name.
func defaultPostgresDatabase(flavor string) string {
	switch flavor {
	case "kingbase":
		return "test"
	case "opengauss":
		return "postgres"
	case "highgo":
		return "highgo"
	case "gbase8a":
		return "gbase"
	case "shentong":
		return "OSRDB"
	default:
		return "postgres"
	}
}

// buildPostgresDSN builds a libpq-style URL. url.UserPassword escapes credentials
// so that special characters cannot break out of the DSN.
func buildPostgresDSN(t ConnectorTarget) string {
	database := t.Database
	if database == "" {
		database = defaultPostgresDatabase(normalizeFlavor(t.Flavor))
	}
	timeoutSeconds := int(t.timeout() / time.Second)
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	query := url.Values{}
	query.Set("connect_timeout", fmt.Sprintf("%d", timeoutSeconds))
	// Prefer TLS when the server offers it, but do not fail when it does not.
	// Vendor defaults vary and a health check must not be the thing that breaks.
	query.Set("sslmode", "prefer")

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(t.Username, t.Password),
		Host:     net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port)),
		Path:     "/" + database,
		RawQuery: query.Encode(),
	}
	return dsn.String()
}

func newPostgresConnector(t ConnectorTarget) (DBConnector, error) {
	db, err := sql.Open("pgx", buildPostgresDSN(t))
	if err != nil {
		return nil, fmt.Errorf("open postgres-compatible connection: %w", err)
	}
	return &postgresConnector{flavor: normalizeFlavor(t.Flavor), db: db}, nil
}

func (c *postgresConnector) Flavor() string { return c.flavor }

func (c *postgresConnector) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *postgresConnector) ServerVersion(ctx context.Context) (string, error) {
	var version string
	if err := c.db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return "", fmt.Errorf("read postgres-compatible server version: %w", err)
	}
	return version, nil
}

func (c *postgresConnector) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}
