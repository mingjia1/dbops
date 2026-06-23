package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type CredentialRepository struct {
	db *Database
}

func NewCredentialRepository(db *Database) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (r *CredentialRepository) Create(ctx context.Context, cred *models.ClusterCredential) error {
	_, err := r.db.Pool.ExecContext(ctx,
		`INSERT INTO cluster_credentials (id, cluster_id, account_type, username, password_enc, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.ClusterID, cred.AccountType, cred.Username, cred.PasswordEnc, cred.CreatedAt,
	)
	return err
}

func (r *CredentialRepository) GetByClusterAndType(ctx context.Context, clusterID, accountType string) (*models.ClusterCredential, error) {
	row := r.db.Pool.QueryRowContext(ctx,
		`SELECT id, cluster_id, account_type, username, password_enc, created_at, rotated_at
		 FROM cluster_credentials WHERE cluster_id = ? AND account_type = ?`,
		clusterID, accountType,
	)
	var cred models.ClusterCredential
	var rotatedAt *time.Time
	err := row.Scan(&cred.ID, &cred.ClusterID, &cred.AccountType, &cred.Username, &cred.PasswordEnc, &cred.CreatedAt, &rotatedAt)
	if err != nil {
		return nil, fmt.Errorf("credential not found: %w", err)
	}
	cred.RotatedAt = rotatedAt
	return &cred, nil
}

func (r *CredentialRepository) ListByCluster(ctx context.Context, clusterID string) ([]models.ClusterCredential, error) {
	rows, err := r.db.Pool.QueryContext(ctx,
		`SELECT id, cluster_id, account_type, username, password_enc, created_at, rotated_at
		 FROM cluster_credentials WHERE cluster_id = ? ORDER BY account_type`,
		clusterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ClusterCredential
	for rows.Next() {
		var cred models.ClusterCredential
		var rotatedAt *time.Time
		if err := rows.Scan(&cred.ID, &cred.ClusterID, &cred.AccountType, &cred.Username, &cred.PasswordEnc, &cred.CreatedAt, &rotatedAt); err != nil {
			return nil, err
		}
		cred.RotatedAt = rotatedAt
		out = append(out, cred)
	}
	return out, rows.Err()
}

func (r *CredentialRepository) UpdatePassword(ctx context.Context, id, passwordEnc string) error {
	now := time.Now()
	_, err := r.db.Pool.ExecContext(ctx,
		`UPDATE cluster_credentials SET password_enc = ?, rotated_at = ? WHERE id = ?`,
		passwordEnc, now, id,
	)
	return err
}

func (r *CredentialRepository) DeleteByCluster(ctx context.Context, clusterID string) error {
	_, err := r.db.Pool.ExecContext(ctx,
		`DELETE FROM cluster_credentials WHERE cluster_id = ?`,
		clusterID,
	)
	return err
}
