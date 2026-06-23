package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
		INSERT INTO alert_rules (id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, expression, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.ID, rule.Name, rule.Metric, rule.Condition, rule.Threshold,
		rule.DurationSeconds, rule.Severity, rule.NotificationChannels,
		rule.Expression, rule.CreatedAt, rule.UpdatedAt)

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "expression") {
			if addErr := r.ensureAlertRuleExpressionColumn(ctx); addErr == nil {
				_, err = r.db.Pool.ExecContext(ctx, query,
					rule.ID, rule.Name, rule.Metric, rule.Condition, rule.Threshold,
					rule.DurationSeconds, rule.Severity, rule.NotificationChannels,
					rule.Expression, rule.CreatedAt, rule.UpdatedAt)
			}
		}
	}
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
			duration_seconds = ?, severity = ?, notification_channels = ?,
			expression = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		rule.Name, rule.Metric, rule.Condition, rule.Threshold,
		rule.DurationSeconds, rule.Severity, rule.NotificationChannels,
		rule.Expression, rule.UpdatedAt, rule.ID)

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "expression") {
			if addErr := r.ensureAlertRuleExpressionColumn(ctx); addErr == nil {
				_, err = r.db.Pool.ExecContext(ctx, query,
					rule.Name, rule.Metric, rule.Condition, rule.Threshold,
					rule.DurationSeconds, rule.Severity, rule.NotificationChannels,
					rule.Expression, rule.UpdatedAt, rule.ID)
			}
		}
	}
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
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, expression, created_at, updated_at
		FROM alert_rules WHERE id = ?
	`

	var rule models.AlertRule
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
		&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
		&rule.Expression, &rule.CreatedAt, &rule.UpdatedAt)

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
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, expression, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "expression") {
			if addErr := r.ensureAlertRuleExpressionColumn(ctx); addErr == nil {
				rows, err = r.db.Pool.QueryContext(ctx, query, limit, offset)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list alert rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.AlertRule, 0)
	for rows.Next() {
		var rule models.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
			&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
			&rule.Expression, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (r *AlertRuleRepository) ensureAlertRuleExpressionColumn(ctx context.Context) error {
	if r.db == nil || r.db.Pool == nil {
		return nil
	}
	_, err := r.db.Pool.ExecContext(ctx, `ALTER TABLE alert_rules ADD COLUMN expression TEXT DEFAULT ''`)
	if err != nil && !isAlreadyExistsError(err) {
		return err
	}
	return nil
}

func (r *AlertRuleRepository) GetActiveAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	if r.db.Pool == nil {
		return []models.AlertRule{}, nil
	}

	query := `
		SELECT id, name, metric, ` + "`condition`" + `, threshold, duration_seconds, severity, notification_channels, expression, created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.QueryContext(ctx, query)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "expression") {
			if addErr := r.ensureAlertRuleExpressionColumn(ctx); addErr == nil {
				rows, err = r.db.Pool.QueryContext(ctx, query)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active alert rules: %w", err)
	}
	defer rows.Close()

	rules := make([]models.AlertRule, 0)
	for rows.Next() {
		var rule models.AlertRule
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Metric, &rule.Condition, &rule.Threshold,
			&rule.DurationSeconds, &rule.Severity, &rule.NotificationChannels,
			&rule.Expression, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// CreateAlertRecord P0-4: 真实写入 alert_records 表, 不再是 no-op mock.
func (r *AlertRuleRepository) CreateAlertRecord(ctx context.Context, rec *models.AlertRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO alert_records (id, rule_id, instance_id, triggered_at, resolved_at, status, severity, value, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.ID, rec.RuleID, rec.InstanceID, rec.TriggeredAt, rec.ResolvedAt,
		rec.Status, rec.Severity, rec.Value, rec.Message, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert alert record: %w", err)
	}
	return nil
}

// ListAlertHistory P0-4: 真实读 alert_records, 不再是写死 2 条.
func (r *AlertRuleRepository) ListAlertHistory(ctx context.Context, filter AlertHistoryFilter) ([]models.AlertRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	// P1-2: 之前 SQL 没应用 filter, 注释承认 "暂时用最常见条件". 客户传 instance_id 也返全表.
	// 修: 动态拼 WHERE, 过滤条件全用 ? 占位符 (防注入).
	whereClauses := []string{}
	args := []interface{}{}
	if filter.InstanceID != "" {
		whereClauses = append(whereClauses, "instance_id = ?")
		args = append(args, filter.InstanceID)
	}
	if filter.RuleID != "" {
		whereClauses = append(whereClauses, "rule_id = ?")
		args = append(args, filter.RuleID)
	}
	if filter.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, filter.Status)
	}
	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)

	query := `SELECT id, rule_id, instance_id, triggered_at, resolved_at, status, severity, value, message, created_at
		FROM alert_records` + where + ` ORDER BY triggered_at DESC LIMIT ? OFFSET ?`

	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert history: %w", err)
	}
	defer rows.Close()
	out := make([]models.AlertRecord, 0)
	for rows.Next() {
		var rec models.AlertRecord
		var resolvedAt sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.RuleID, &rec.InstanceID, &rec.TriggeredAt, &resolvedAt,
			&rec.Status, &rec.Severity, &rec.Value, &rec.Message, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			t := resolvedAt.Time
			rec.ResolvedAt = &t
		}
		out = append(out, rec)
	}
	return out, nil
}

// --- AlertTemplateRepository ---

type AlertTemplateRepository struct {
	db *Database
}

func NewAlertTemplateRepository(db *Database) *AlertTemplateRepository {
	return &AlertTemplateRepository{db: db}
}

func (r *AlertTemplateRepository) Create(ctx context.Context, tpl *models.AlertTemplate) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	tpl.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO alert_templates (id, name, category, metric, `+"`condition`"+`, threshold, expression, duration_seconds, severity, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tpl.ID, tpl.Name, tpl.Category, tpl.Metric, tpl.Condition, tpl.Threshold,
		tpl.Expression, tpl.DurationSeconds, tpl.Severity, tpl.Message, tpl.CreatedAt)
	return err
}

func (r *AlertTemplateRepository) List(ctx context.Context, category string) ([]models.AlertTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.AlertTemplate{}, nil
	}
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = r.db.Pool.QueryContext(ctx,
			`SELECT id, name, category, metric, `+"`condition`"+`, threshold, expression, duration_seconds, severity, message, created_at
			 FROM alert_templates WHERE category = ? ORDER BY name`, category)
	} else {
		rows, err = r.db.Pool.QueryContext(ctx,
			`SELECT id, name, category, metric, `+"`condition`"+`, threshold, expression, duration_seconds, severity, message, created_at
			 FROM alert_templates ORDER BY category, name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AlertTemplate, 0)
	for rows.Next() {
		var t models.AlertTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Metric, &t.Condition,
			&t.Threshold, &t.Expression, &t.DurationSeconds, &t.Severity, &t.Message, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *AlertTemplateRepository) GetByID(ctx context.Context, id string) (*models.AlertTemplate, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var t models.AlertTemplate
	err := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, name, category, metric, `+"`condition`"+`, threshold, expression, duration_seconds, severity, message, created_at
		 FROM alert_templates WHERE id = ?`, id).Scan(
		&t.ID, &t.Name, &t.Category, &t.Metric, &t.Condition,
		&t.Threshold, &t.Expression, &t.DurationSeconds, &t.Severity, &t.Message, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("alert template not found")
		}
		return nil, err
	}
	return &t, nil
}

func (r *AlertTemplateRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM alert_templates WHERE id = ?", id)
	return err
}

// AlertHistoryFilter 解耦 services.AlertHistoryFilter, 避免 import 循环.
type AlertHistoryFilter struct {
	InstanceID string
	RuleID     string
	Status     string
	Limit      int
	Offset     int
}
