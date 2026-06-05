package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type BackupRepository struct {
	db *Database
}

func NewBackupRepository(db *Database) *BackupRepository {
	return &BackupRepository{db: db}
}

func (r *BackupRepository) CreatePolicy(ctx context.Context, policy *models.BackupPolicy) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	policy.ID = uuid.New().String()

	query := `
		INSERT INTO backup_policies (id, instance_id, backup_type, schedule, retention_days, storage_type, storage_path, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		policy.ID, policy.InstanceID, policy.BackupType, policy.Schedule, policy.RetentionDays,
		policy.StorageType, policy.StoragePath, policy.Enabled, policy.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create backup policy: %w", err)
	}

	return nil
}

func (r *BackupRepository) GetPolicyByID(ctx context.Context, id string) (*models.BackupPolicy, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, instance_id, backup_type, schedule, retention_days, storage_type, storage_path, enabled, created_at, updated_at
		FROM backup_policies WHERE id = ?
	`

	policy := &models.BackupPolicy{}
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&policy.ID, &policy.InstanceID, &policy.BackupType, &policy.Schedule, &policy.RetentionDays,
		&policy.StorageType, &policy.StoragePath, &policy.Enabled, &policy.CreatedAt, &policy.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("backup policy not found")
		}
		return nil, fmt.Errorf("failed to get backup policy: %w", err)
	}

	return policy, nil
}

func (r *BackupRepository) CreateRecord(ctx context.Context, record *models.BackupRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	record.ID = uuid.New().String()

	query := `
		INSERT INTO backup_records (id, policy_id, instance_id, backup_type, started_at, completed_at, status, file_path, file_size, checksum, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		record.ID, record.PolicyID, record.InstanceID, record.BackupType, record.StartedAt, record.CompletedAt,
		record.Status, record.FilePath, record.FileSize, record.Checksum, record.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create backup record: %w", err)
	}

	return nil
}

func (r *BackupRepository) ListRecords(ctx context.Context, instanceID string, limit, offset int) ([]models.BackupRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, policy_id, instance_id, backup_type, started_at, completed_at, status, file_path, file_size, checksum, created_at
		FROM backup_records WHERE instance_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Pool.QueryContext(ctx, query, instanceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list backup records: %w", err)
	}
	defer rows.Close()

	var records []models.BackupRecord
	for rows.Next() {
		var record models.BackupRecord
		if err := rows.Scan(&record.ID, &record.PolicyID, &record.InstanceID, &record.BackupType,
			&record.StartedAt, &record.CompletedAt, &record.Status, &record.FilePath, &record.FileSize, &record.Checksum, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}
