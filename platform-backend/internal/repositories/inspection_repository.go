package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type InspectionTemplateRepository struct {
	db *Database
}

type InspectionReportRepository struct {
	db *Database
}

func NewInspectionTemplateRepository(db *Database) *InspectionTemplateRepository {
	return &InspectionTemplateRepository{db: db}
}

func NewInspectionReportRepository(db *Database) *InspectionReportRepository {
	return &InspectionReportRepository{db: db}
}

// --- Template ---

func (r *InspectionTemplateRepository) Create(ctx context.Context, t *models.InspectionTemplate) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	t.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO inspection_templates (id, name, category, config, schedule, recipients, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Name, t.Category, t.Config, t.Schedule, t.Recipients, t.Enabled, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *InspectionTemplateRepository) List(ctx context.Context, category string) ([]models.InspectionTemplate, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, category, config, schedule, recipients, enabled, created_at, updated_at
			FROM inspection_templates WHERE category = ? ORDER BY name`, category)
	} else {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, category, config, schedule, recipients, enabled, created_at, updated_at
			FROM inspection_templates ORDER BY category, name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.InspectionTemplate, 0)
	for rows.Next() {
		var t models.InspectionTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Config, &t.Schedule,
			&t.Recipients, &t.Enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *InspectionTemplateRepository) GetByID(ctx context.Context, id string) (*models.InspectionTemplate, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var t models.InspectionTemplate
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, category, config, schedule, recipients, enabled, created_at, updated_at
		FROM inspection_templates WHERE id = ?`, id).Scan(
		&t.ID, &t.Name, &t.Category, &t.Config, &t.Schedule,
		&t.Recipients, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("inspection template not found")
		}
		return nil, err
	}
	return &t, nil
}

func (r *InspectionTemplateRepository) Update(ctx context.Context, t *models.InspectionTemplate) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		UPDATE inspection_templates SET name = ?, category = ?, config = ?, schedule = ?, recipients = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		t.Name, t.Category, t.Config, t.Schedule, t.Recipients, t.Enabled, t.UpdatedAt, t.ID)
	return err
}

func (r *InspectionTemplateRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM inspection_templates WHERE id = ?", id)
	return err
}

// --- Report ---

func (r *InspectionReportRepository) Create(ctx context.Context, rep *models.InspectionReport) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	rep.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO inspection_reports (id, template_id, instance_id, status, summary, details, score, generated_at, sent_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rep.ID, rep.TemplateID, rep.InstanceID, rep.Status, rep.Summary, rep.Details,
		rep.Score, rep.GeneratedAt, rep.SentAt, rep.CreatedAt)
	return err
}

func (r *InspectionReportRepository) List(ctx context.Context, templateID string, limit, offset int) ([]models.InspectionReport, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	args := []interface{}{limit, offset}
	where := ""
	if templateID != "" {
		where = " WHERE template_id = ?"
		args = append([]interface{}{templateID}, args...)
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, template_id, instance_id, status, summary, details, score, generated_at, sent_at, created_at
		FROM inspection_reports`+where+` ORDER BY generated_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.InspectionReport, 0)
	for rows.Next() {
		var rep models.InspectionReport
		var sentAt sql.NullTime
		if err := rows.Scan(&rep.ID, &rep.TemplateID, &rep.InstanceID, &rep.Status, &rep.Summary, &rep.Details,
			&rep.Score, &rep.GeneratedAt, &sentAt, &rep.CreatedAt); err != nil {
			return nil, err
		}
		if sentAt.Valid {
			rep.SentAt = &sentAt.Time
		}
		out = append(out, rep)
	}
	return out, nil
}

func (r *InspectionReportRepository) GetByID(ctx context.Context, id string) (*models.InspectionReport, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rep models.InspectionReport
	var sentAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, template_id, instance_id, status, summary, details, score, generated_at, sent_at, created_at
		FROM inspection_reports WHERE id = ?`, id).Scan(
		&rep.ID, &rep.TemplateID, &rep.InstanceID, &rep.Status, &rep.Summary, &rep.Details,
		&rep.Score, &rep.GeneratedAt, &sentAt, &rep.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("inspection report not found")
		}
		return nil, err
	}
	if sentAt.Valid {
		rep.SentAt = &sentAt.Time
	}
	return &rep, nil
}

func (r *InspectionReportRepository) Update(ctx context.Context, rep *models.InspectionReport) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		UPDATE inspection_reports SET status = ?, summary = ?, details = ?, score = ?, sent_at = ? WHERE id = ?`,
		rep.Status, rep.Summary, rep.Details, rep.Score, rep.SentAt, rep.ID)
	return err
}

func (r *InspectionReportRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM inspection_reports WHERE id = ?", id)
	return err
}

func (r *InspectionReportRepository) GetLatestByTemplate(ctx context.Context, templateID string) (*models.InspectionReport, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rep models.InspectionReport
	var sentAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, template_id, instance_id, status, summary, details, score, generated_at, sent_at, created_at
		FROM inspection_reports WHERE template_id = ? ORDER BY generated_at DESC LIMIT 1`, templateID).Scan(
		&rep.ID, &rep.TemplateID, &rep.InstanceID, &rep.Status, &rep.Summary, &rep.Details,
		&rep.Score, &rep.GeneratedAt, &sentAt, &rep.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if sentAt.Valid {
		rep.SentAt = &sentAt.Time
	}
	return &rep, nil
}

// Ensure compile-time interface for time import
var _ = time.Now
