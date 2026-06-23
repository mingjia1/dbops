package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type EscalationRepository struct {
	db *Database
}

type SilenceRepository struct {
	db *Database
}

func NewEscalationRepository(db *Database) *EscalationRepository {
	return &EscalationRepository{db: db}
}

func NewSilenceRepository(db *Database) *SilenceRepository {
	return &SilenceRepository{db: db}
}

// --- Escalation ---

func (r *EscalationRepository) Create(ctx context.Context, e *models.AlertEscalation) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	e.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO escalation_rules (id, rule_id, level, severity, interval_sec, notify_channels, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.RuleID, e.Level, e.Severity, e.IntervalSec, e.NotifyChannels, e.CreatedAt, e.UpdatedAt)
	return err
}

func (r *EscalationRepository) ListByRuleID(ctx context.Context, ruleID string) ([]models.AlertEscalation, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, rule_id, level, severity, interval_sec, notify_channels, created_at, updated_at
		FROM escalation_rules WHERE rule_id = ? ORDER BY level ASC`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AlertEscalation, 0)
	for rows.Next() {
		var e models.AlertEscalation
		if err := rows.Scan(&e.ID, &e.RuleID, &e.Level, &e.Severity, &e.IntervalSec, &e.NotifyChannels, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *EscalationRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM escalation_rules WHERE id = ?", id)
	return err
}

func (r *EscalationRepository) GetByID(ctx context.Context, id string) (*models.AlertEscalation, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var e models.AlertEscalation
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, rule_id, level, severity, interval_sec, notify_channels, created_at, updated_at
		FROM escalation_rules WHERE id = ?`, id).Scan(
		&e.ID, &e.RuleID, &e.Level, &e.Severity, &e.IntervalSec, &e.NotifyChannels, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("escalation rule not found")
		}
		return nil, err
	}
	return &e, nil
}

// --- Silence ---

func (r *SilenceRepository) Create(ctx context.Context, s *models.AlertSilence) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	s.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO alert_silences (id, name, rule_ids, match_expr, start_at, end_at, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Name, s.RuleIDs, s.MatchExpr, s.StartAt, s.EndAt, s.Enabled, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *SilenceRepository) List(ctx context.Context) ([]models.AlertSilence, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, name, rule_ids, match_expr, start_at, end_at, enabled, created_at, updated_at
		FROM alert_silences ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AlertSilence, 0)
	for rows.Next() {
		var s models.AlertSilence
		if err := rows.Scan(&s.ID, &s.Name, &s.RuleIDs, &s.MatchExpr, &s.StartAt, &s.EndAt, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *SilenceRepository) GetByID(ctx context.Context, id string) (*models.AlertSilence, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var s models.AlertSilence
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, rule_ids, match_expr, start_at, end_at, enabled, created_at, updated_at
		FROM alert_silences WHERE id = ?`, id).Scan(
		&s.ID, &s.Name, &s.RuleIDs, &s.MatchExpr, &s.StartAt, &s.EndAt, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("silence rule not found")
		}
		return nil, err
	}
	return &s, nil
}

func (r *SilenceRepository) Update(ctx context.Context, s *models.AlertSilence) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		UPDATE alert_silences SET name = ?, rule_ids = ?, match_expr = ?, start_at = ?, end_at = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		s.Name, s.RuleIDs, s.MatchExpr, s.StartAt, s.EndAt, s.Enabled, s.UpdatedAt, s.ID)
	return err
}

func (r *SilenceRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM alert_silences WHERE id = ?", id)
	return err
}
