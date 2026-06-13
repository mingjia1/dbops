package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouse struct {
	conn driver.Conn
}

func NewClickHouse(url string) (*ClickHouse, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
		},
	})
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