package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type DiagnosisRepository struct {
	db *Database
}

type SQLAdviceRepository struct {
	db *Database
}

func NewDiagnosisRepository(db *Database) *DiagnosisRepository {
	return &DiagnosisRepository{db: db}
}

func NewSQLAdviceRepository(db *Database) *SQLAdviceRepository {
	return &SQLAdviceRepository{db: db}
}

// --- Diagnosis ---

func (r *DiagnosisRepository) Create(ctx context.Context, d *models.DiagnosisRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	d.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO diagnosis_records (id, instance_id, status, summary, details, score, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.InstanceID, d.Status, d.Summary, d.Details, d.Score, d.CreatedAt)
	return err
}

func (r *DiagnosisRepository) List(ctx context.Context, instanceID string, limit, offset int) ([]models.DiagnosisRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	args := []interface{}{limit, offset}
	where := ""
	if instanceID != "" {
		where = " WHERE instance_id = ?"
		args = append([]interface{}{instanceID}, args...)
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, instance_id, status, summary, details, score, created_at
		FROM diagnosis_records`+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.DiagnosisRecord, 0)
	for rows.Next() {
		var d models.DiagnosisRecord
		if err := rows.Scan(&d.ID, &d.InstanceID, &d.Status, &d.Summary, &d.Details, &d.Score, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *DiagnosisRepository) GetByID(ctx context.Context, id string) (*models.DiagnosisRecord, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var d models.DiagnosisRecord
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, instance_id, status, summary, details, score, created_at
		FROM diagnosis_records WHERE id = ?`, id).Scan(
		&d.ID, &d.InstanceID, &d.Status, &d.Summary, &d.Details, &d.Score, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("diagnosis record not found")
		}
		return nil, err
	}
	return &d, nil
}

// --- SQLAdvice ---

func (r *SQLAdviceRepository) Create(ctx context.Context, a *models.SQLAdvice) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	a.ID = uuid.New().String()
	_, err := r.db.Pool.ExecContext(ctx, `
		INSERT INTO sql_advices (id, sql_text, `+"`explain`"+`, advice, score, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, a.ID, a.SQLText, a.Explain, a.Advice, a.Score, a.CreatedAt)
	return err
}

func (r *SQLAdviceRepository) List(ctx context.Context, limit, offset int) ([]models.SQLAdvice, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := r.db.Pool.QueryContext(ctx, `
		SELECT id, sql_text, `+"`explain`"+`, advice, score, created_at
		FROM sql_advices ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.SQLAdvice, 0)
	for rows.Next() {
		var a models.SQLAdvice
		if err := rows.Scan(&a.ID, &a.SQLText, &a.Explain, &a.Advice, &a.Score, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *SQLAdviceRepository) GetByID(ctx context.Context, id string) (*models.SQLAdvice, error) {
	if r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	var a models.SQLAdvice
	err := r.db.Pool.QueryRowContext(ctx, `
		SELECT id, sql_text, `+"`explain`"+`, advice, score, created_at
		FROM sql_advices WHERE id = ?`, id).Scan(
		&a.ID, &a.SQLText, &a.Explain, &a.Advice, &a.Score, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("sql advice not found")
		}
		return nil, err
	}
	return &a, nil
}
