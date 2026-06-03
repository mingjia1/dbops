package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type AlertRuleRepository struct {
	db *Database
}

func NewAlertRuleRepository(db *Database) *AlertRuleRepository {
	return &AlertRuleRepository{db: db}
}

func (r *AlertRuleRepository) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	rule.ID = uuid.New().String()

	query := `
		INSERT INTO alert_rules (id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.ID, rule.Name, rule.Metric, rule.Condition, rule.Threshold,
		rule.DurationSeconds, rule.Severity, rule.NotificationChannels,
		rule.CreatedAt, rule.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create alert rule: %w", err)
	}

	return nil
}

func (r *AlertRuleRepository) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	query := `
		UPDATE alert_rules SET
			name = ?, metric = ?, ` + "`condition`" + ` = ?, threshold = ?,
			duration_seconds = ?, severity = ?, notification_channels = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.Name, rule.Metric, rule.Condition, rule.Threshold,
		rule.DurationSeconds, rule.Severity, rule.NotificationChannels, rule.UpdatedAt,
		rule.ID)

	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}

	return nil
}

func (r *AlertRuleRepository) DeleteAlertRule(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}

	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM alert_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete alert rule: %w", err)
	}

	return nil
}

func (r *AlertRuleRepository) GetAlertRuleByID(ctx context.Context, id string) (*models.AlertRule, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available in standalone mode")
	}

	query := `
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules WHERE id = ?
	`

	var rule models.AlertRule
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
		&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
		&rule.CreatedAt, &rule.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("alert rule not found")
		}
		return nil, fmt.Errorf("failed to get alert rule: %w", err)
	}

	return &rule, nil
}

func (r *AlertRuleRepository) ListAlertRules(ctx context.Context, limit, offset int) ([]models.AlertRule, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.AlertRule{}, nil
	}

	query := `
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var rule models.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
			&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
			&rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (r *AlertRuleRepository) GetActiveAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	if r.db.Pool == nil {
		return []models.AlertRule{}, nil
	}

	query := `
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active alert rules: %w", err)
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var rule models.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
			&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
			&rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}
