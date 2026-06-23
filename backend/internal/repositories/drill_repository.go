package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type DrillRepository struct {
	db *Database
}

type DrillReportRepository struct {
	db *Database
}

func NewDrillRepository(db *Database) *DrillRepository {
	return &DrillRepository{db: db}
}

func NewDrillReportRepository(db *Database) *DrillReportRepository {
	return &DrillReportRepository{db: db}
}

// --- HADrill ---

func (r *DrillRepository) Create(ctx context.Context, d *models.HADrill) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	d.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO ha_drills (id, name, description, plan, status, result, score, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.Name, d.Description, d.Plan, d.Status, d.Result, d.Score,
		d.StartedAt, d.CompletedAt, d.CreatedAt, d.UpdatedAt)
	return err
}

func (r *DrillRepository) List(ctx context.Context, status string) ([]models.HADrill, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, description, plan, status, result, score, started_at, completed_at, created_at, updated_at
			FROM ha_drills WHERE status = ? ORDER BY created_at DESC`, status)
	} else {
		rows, err = r.db.Pool.QueryContext(ctx, `
			SELECT id, name, description, plan, status, result, score, started_at, completed_at, created_at, updated_at
			FROM ha_drills ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.HADrill, 0)
	for rows.Next() {
		var d models.HADrill
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.Plan, &d.Status, &d.Result,
			&d.Score, &startedAt, &completedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			d.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			d.CompletedAt = &completedAt.Time
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *DrillRepository) GetByID(ctx context.Context, id string) (*models.HADrill, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var d models.HADrill
	var startedAt, completedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, name, description, plan, status, result, score, started_at, completed_at, created_at, updated_at
		FROM ha_drills WHERE id = ?`, id).Scan(
		&d.ID, &d.Name, &d.Description, &d.Plan, &d.Status, &d.Result,
		&d.Score, &startedAt, &completedAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("drill not found")
		}
		return nil, err
	}
	if startedAt.Valid {
		d.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		d.CompletedAt = &completedAt.Time
	}
	return &d, nil
}

func (r *DrillRepository) Update(ctx context.Context, d *models.HADrill) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `
		UPDATE ha_drills SET name = ?, description = ?, plan = ?, status = ?, result = ?, score = ?, started_at = ?, completed_at = ?, updated_at = ?
		WHERE id = ?`,
		d.Name, d.Description, d.Plan, d.Status, d.Result, d.Score,
		d.StartedAt, d.CompletedAt, d.UpdatedAt, d.ID)
	return err
}

func (r *DrillRepository) Delete(ctx context.Context, id string) error {
	if r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, "DELETE FROM ha_drills WHERE id = ?", id)
	return err
}

// --- DrillReport ---

func (r *DrillReportRepository) Create(ctx context.Context, rep *models.HADrillReport) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	rep.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO ha_drill_reports (id, drill_id, summary, timeline, findings, score, generated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, rep.ID, rep.DrillID, rep.Summary, rep.Timeline, rep.Findings, rep.Score, rep.GeneratedAt, rep.CreatedAt)
	return err
}

func (r *DrillReportRepository) GetByDrillID(ctx context.Context, drillID string) (*models.HADrillReport, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var rep models.HADrillReport
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, drill_id, summary, timeline, findings, score, generated_at, created_at
		FROM ha_drill_reports WHERE drill_id = ?`, drillID).Scan(
		&rep.ID, &rep.DrillID, &rep.Summary, &rep.Timeline, &rep.Findings, &rep.Score, &rep.GeneratedAt, &rep.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rep, nil
}
