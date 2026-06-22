package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouse struct {
	conn driver.Conn
}

// ClickHouseConfig holds ClickHouse connection configuration
type ClickHouseConfig struct {
	Addr         string
	Database     string
	Username     string
	Password     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// ParseClickHouseURL parses a ClickHouse URL into config
// Supported formats:
//   - clickhouse://user:password@host:port/database
//   - clickhouse://user@host:port/database
//   - clickhouse://host:port/database
//   - clickhouse://host:port (defaults to "default" database)
func ParseClickHouseURL(rawURL string) (*ClickHouseConfig, error) {
	if rawURL == "" {
		return &ClickHouseConfig{
			Addr:     "localhost:9000",
			Database: "default",
			Username: "default",
		}, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid clickhouse URL: %w", err)
	}

	cfg := &ClickHouseConfig{
		Addr:     u.Host,
		Database: "default",
		Username: "default",
	}

	if u.User != nil {
		cfg.Username = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			cfg.Password = pass
		}
	}

	if u.Path != "" && u.Path != "/" {
		cfg.Database = strings.TrimPrefix(u.Path, "/")
	}

	return cfg, nil
}

func NewClickHouse(url string, configOverride ...*ClickHouseConfig) (*ClickHouse, error) {
	cfg, err := ParseClickHouseURL(url)
	if err != nil {
		return nil, err
	}

	// Apply overrides if provided
	if len(configOverride) > 0 && configOverride[0] != nil {
		oc := configOverride[0]
		if oc.MaxOpenConns > 0 {
			cfg.MaxOpenConns = oc.MaxOpenConns
		}
		if oc.MaxIdleConns > 0 {
			cfg.MaxIdleConns = oc.MaxIdleConns
		}
		if oc.MaxLifetime > 0 {
			cfg.MaxLifetime = oc.MaxLifetime
		}
	}

	opts := &clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	ch := &ClickHouse{
		conn: conn,
	}

	if err := ch.initTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}

	return ch, nil
}

func (ch *ClickHouse) initTables(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS metrics (
			instance_id String,
			metric_name String,
			value Float64,
			timestamp DateTime64(3)
		) ENGINE = MergeTree()
		ORDER BY (instance_id, metric_name, timestamp)
	`

	if err := ch.conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create metrics table: %w", err)
	}

	return nil
}

func (ch *ClickHouse) WriteMetric(ctx context.Context, instanceID, metricName string, value float64, timestamp time.Time) error {
	if err := ch.conn.Exec(ctx, `
		INSERT INTO metrics (instance_id, metric_name, value, timestamp)
		VALUES (?, ?, ?, ?)
	`, instanceID, metricName, value, timestamp); err != nil {
		return fmt.Errorf("failed to write metric: %w", err)
	}

	return nil
}

func (ch *ClickHouse) QueryMetrics(ctx context.Context, instanceID string, metricNames []string, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	rows, err := ch.conn.Query(ctx, `
		SELECT
			metric_name AS name,
			value,
			timestamp
		FROM metrics
		WHERE instance_id = ?
			AND metric_name IN (?)
			AND timestamp >= ?
			AND timestamp <= ?
		ORDER BY timestamp ASC
	`, instanceID, metricNames, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var name string
		var value float64
		var timestamp time.Time

		if err := rows.Scan(&name, &value, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		results = append(results, map[string]interface{}{
			"name":      name,
			"value":     value,
			"timestamp": timestamp,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return results, nil
}

func (ch *ClickHouse) Close() error {
	if ch.conn != nil {
		return ch.conn.Close()
	}
	return nil
}

// HealthCheck checks if ClickHouse connection is alive
func (ch *ClickHouse) HealthCheck(ctx context.Context) error {
	if ch.conn == nil {
		return fmt.Errorf("clickhouse connection not initialized")
	}
	return ch.conn.Ping(ctx)
}