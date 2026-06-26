package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
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

func (r *BackupRepository) ListPolicies(ctx context.Context, instanceID string, limit, offset int) ([]models.BackupPolicy, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, instance_id, backup_type, schedule, retention_days, storage_type, storage_path, enabled, created_at, updated_at
		FROM backup_policies
	`
	args := []interface{}{}
	if instanceID != "" {
		query += ` WHERE instance_id = ?`
		args = append(args, instanceID)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list backup policies: %w", err)
	}
	defer rows.Close()

	policies := make([]models.BackupPolicy, 0)
	for rows.Next() {
		var policy models.BackupPolicy
		if err := rows.Scan(
			&policy.ID, &policy.InstanceID, &policy.BackupType, &policy.Schedule, &policy.RetentionDays,
			&policy.StorageType, &policy.StoragePath, &policy.Enabled, &policy.CreatedAt, &policy.UpdatedAt,
		); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (r *BackupRepository) UpdatePolicy(ctx context.Context, policy *models.BackupPolicy) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if policy == nil || policy.ID == "" {
		return fmt.Errorf("backup policy id is required")
	}
	query := `
		UPDATE backup_policies
		SET instance_id = ?, backup_type = ?, schedule = ?, retention_days = ?, storage_type = ?, storage_path = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := r.db.Pool.ExecContext(ctx, query,
		policy.InstanceID, policy.BackupType, policy.Schedule, policy.RetentionDays, policy.StorageType,
		policy.StoragePath, policy.Enabled, policy.UpdatedAt, policy.ID)
	if err != nil {
		return fmt.Errorf("failed to update backup policy: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("backup policy not found")
	}
	return nil
}

func (r *BackupRepository) DeletePolicy(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx,
		`DELETE FROM backup_policies WHERE id = ? AND (SELECT COUNT(*) FROM backup_records WHERE policy_id = ?) = 0`,
		id, id)
	if err != nil {
		return fmt.Errorf("failed to delete backup policy: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("backup policy not found or has backup records and cannot be deleted")
	}
	return nil
}

func (r *BackupRepository) CreateRecord(ctx context.Context, record *models.BackupRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	record.ID = uuid.New().String()
	var policyID interface{} = record.PolicyID
	if record.PolicyID == "" {
		policyID = nil
	}

	query := `
		INSERT INTO backup_records (id, policy_id, instance_id, backup_type, task_id, started_at, completed_at, status, message, file_path, file_size, checksum, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Pool.ExecContext(ctx, query,
		record.ID, policyID, record.InstanceID, record.BackupType, record.TaskID, record.StartedAt, record.CompletedAt,
		record.Status, record.Message, record.FilePath, record.FileSize, record.Checksum, record.CreatedAt)

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
		SELECT id, policy_id, instance_id, backup_type, COALESCE(task_id, ''), started_at, completed_at, status, COALESCE(message, ''), file_path, file_size, checksum, created_at
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
		var policyID sql.NullString
		if err := rows.Scan(&record.ID, &policyID, &record.InstanceID, &record.BackupType,
			&record.TaskID, &record.StartedAt, &record.CompletedAt, &record.Status, &record.Message, &record.FilePath, &record.FileSize, &record.Checksum, &record.CreatedAt); err != nil {
			return nil, err
		}
		if policyID.Valid {
			record.PolicyID = policyID.String
		}
		records = append(records, record)
	}

	return records, nil
}

func (r *BackupRepository) GetRecordByID(ctx context.Context, id string) (*models.BackupRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, policy_id, instance_id, backup_type, task_id, started_at, completed_at, status, message, file_path, file_size, checksum, created_at
		FROM backup_records WHERE id = ?
	`
	record := &models.BackupRecord{}
	var policyID sql.NullString
	var taskID sql.NullString
	var message sql.NullString
	var completedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&record.ID, &policyID, &record.InstanceID, &record.BackupType,
		&taskID, &record.StartedAt, &completedAt, &record.Status, &message, &record.FilePath,
		&record.FileSize, &record.Checksum, &record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("backup record not found")
		}
		return nil, fmt.Errorf("failed to get backup record: %w", err)
	}
	if policyID.Valid {
		record.PolicyID = policyID.String
	}
	if taskID.Valid {
		record.TaskID = taskID.String
	}
	if message.Valid {
		record.Message = message.String
	}
	if completedAt.Valid {
		record.CompletedAt = completedAt.Time
	}
	return record, nil
}

func (r *BackupRepository) UpdateRecord(ctx context.Context, record *models.BackupRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if record == nil || record.ID == "" {
		return fmt.Errorf("backup record id is required")
	}
	query := `
		UPDATE backup_records
		SET completed_at = ?, status = ?, message = ?, file_path = ?, file_size = ?, checksum = ?
		WHERE id = ?
	`
	res, err := r.db.Pool.ExecContext(ctx, query,
		record.CompletedAt, record.Status, record.Message, record.FilePath, record.FileSize, record.Checksum, record.ID)
	if err != nil {
		return fmt.Errorf("failed to update backup record: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("backup record not found")
	}
	return nil
}

func (r *BackupRepository) DeleteRecord(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `DELETE FROM backup_records WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete backup record: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("backup record not found")
	}
	return nil
}

func (r *BackupRepository) CreateRestoreRecord(ctx context.Context, record *models.RestoreRecord) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	record.ID = uuid.New().String()
	query := `
		INSERT INTO restore_records (id, backup_id, target_instance_id, started_at, completed_at, status, restore_point, restored_tables, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	restorePoint := record.RestorePoint
	if restorePoint.IsZero() || restorePoint.Year() < 1970 {
		restorePoint = time.Time{}
	}
	_, err := r.db.Pool.ExecContext(ctx, query,
		record.ID, record.BackupID, record.TargetInstanceID, record.StartedAt, record.CompletedAt,
		record.Status, restorePoint, record.RestoredTables, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create restore record: %w", err)
	}
	return nil
}

func (r *BackupRepository) LatestCompletedRecord(ctx context.Context, instanceID, backupType string) (*models.BackupRecord, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, policy_id, instance_id, backup_type, task_id, started_at, completed_at, status, message, file_path, file_size, checksum, created_at
		FROM backup_records
		WHERE instance_id = ? AND backup_type = ? AND status = 'completed' AND file_path <> ''
		ORDER BY completed_at DESC, created_at DESC LIMIT 1
	`
	record := &models.BackupRecord{}
	var policyID sql.NullString
	var taskID sql.NullString
	var message sql.NullString
	var completedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, instanceID, backupType).Scan(
		&record.ID, &policyID, &record.InstanceID, &record.BackupType,
		&taskID, &record.StartedAt, &completedAt, &record.Status, &message, &record.FilePath,
		&record.FileSize, &record.Checksum, &record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if policyID.Valid {
		record.PolicyID = policyID.String
	}
	if taskID.Valid {
		record.TaskID = taskID.String
	}
	if message.Valid {
		record.Message = message.String
	}
	if completedAt.Valid {
		record.CompletedAt = completedAt.Time
	}
	return record, nil
}
