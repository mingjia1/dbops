package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		INSERT INTO alert_rules (id, name, metric, condition, threshold, duration_seconds, severity, notification_channels, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
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
			name = $2, metric = $3, condition = $4, threshold = $5,
			duration_seconds = $6, severity = $7, notification_channels = $8, updated_at = $9
		WHERE id = $1
	`
	
	_, err := r.db.Pool.Exec(ctx, query,
		rule.ID, rule.Name, rule.Metric, rule.Condition, rule.Threshold,
		rule.DurationSeconds, rule.Severity, rule.NotificationChannels, rule.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}
	
	return nil
}

func (r *AlertRuleRepository) DeleteAlertRule(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available in standalone mode")
	}
	
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM alert_rules WHERE id = $1", id)
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
		SELECT id, name, metric, condition, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules WHERE id = $1
	`
	
	var rule models.AlertRule
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
		&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
		&rule.CreatedAt, &rule.UpdatedAt)
	
	if err != nil {
		if err == pgx.ErrNoRows {
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
		SELECT id, name, metric, condition, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
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
		SELECT id, name, metric, condition, threshold, duration_seconds, severity, notification_channels, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC
	`
	
	rows, err := r.db.Pool.Query(ctx, query)
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