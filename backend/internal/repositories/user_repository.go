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

type UserRepository struct {
	db *Database
}

func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	query := `
		INSERT INTO users (id, username, password, email, role, status, display_name, phone, source, password_changed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	source := user.Source
	if source == "" {
		source = "local"
	}
	passwordChangedAt := user.PasswordChangedAt
	if passwordChangedAt == nil {
		now := time.Now()
		passwordChangedAt = &now
	}
	_, err := r.db.Pool.ExecContext(ctx, query,
		user.ID, user.Username, user.Password, user.Email, user.Role, user.Status,
		user.DisplayName, user.Phone, source, passwordChangedAt, user.CreatedAt)
	return err
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, display_name, phone, source, last_login_at, last_login_ip, password_changed_at, created_at, updated_at
		FROM users WHERE username = ?
	`
	user := &models.User{}
	var email sql.NullString
	var updatedAt sql.NullTime
	var displayName, phone, source, lastLoginIP sql.NullString
	var lastLoginAt, passwordChangedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Password, &email, &user.Role, &user.Status,
		&displayName, &phone, &source, &lastLoginAt, &lastLoginIP, &passwordChangedAt, &user.CreatedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if email.Valid {
		user.Email = email.String
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	applyUserNulls(user, displayName, phone, source, lastLoginIP, lastLoginAt, passwordChangedAt)
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, display_name, phone, source, last_login_at, last_login_ip, password_changed_at, created_at, updated_at
		FROM users WHERE id = ?
	`
	user := &models.User{}
	var email sql.NullString
	var updatedAt sql.NullTime
	var displayName, phone, source, lastLoginIP sql.NullString
	var lastLoginAt, passwordChangedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Password, &email, &user.Role, &user.Status,
		&displayName, &phone, &source, &lastLoginAt, &lastLoginIP, &passwordChangedAt, &user.CreatedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if email.Valid {
		user.Email = email.String
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	applyUserNulls(user, displayName, phone, source, lastLoginIP, lastLoginAt, passwordChangedAt)
	return user, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, display_name, phone, source, last_login_at, last_login_ip, password_changed_at, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.Pool.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		var email sql.NullString
		var updatedAt sql.NullTime
		var displayName, phone, source, lastLoginIP sql.NullString
		var lastLoginAt, passwordChangedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &email, &u.Role, &u.Status,
			&displayName, &phone, &source, &lastLoginAt, &lastLoginIP, &passwordChangedAt, &u.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		if email.Valid {
			u.Email = email.String
		}
		if updatedAt.Valid {
			u.UpdatedAt = updatedAt.Time
		}
		applyUserNulls(&u, displayName, phone, source, lastLoginIP, lastLoginAt, passwordChangedAt)
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `
		UPDATE users SET username = ?, password = ?, email = ?, role = ?, status = ?, display_name = ?, phone = ?, source = ?, updated_at = ?
		WHERE id = ?
	`
	source := user.Source
	if source == "" {
		source = "local"
	}
	_, err := r.db.Pool.ExecContext(ctx, query,
		user.Username, user.Password, user.Email, user.Role, user.Status, user.DisplayName, user.Phone, source, user.UpdatedAt, user.ID)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	now := time.Now()
	res, err := r.db.Pool.ExecContext(ctx, `UPDATE users SET password = ?, password_changed_at = ?, updated_at = ? WHERE id = ?`, passwordHash, now, now, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateLoginMetadata(ctx context.Context, userID, ip string, at time.Time) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `UPDATE users SET last_login_at = ?, last_login_ip = ?, updated_at = ? WHERE id = ?`, at, ip, at, userID)
	return err
}

func (r *UserRepository) UpdateStatus(ctx context.Context, userID, status string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateAllPasswords(ctx context.Context, passwordHash string) (int64, error) {
	if r.db == nil || r.db.Pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `UPDATE users SET password = ?, updated_at = ?`, passwordHash, time.Now())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func applyUserNulls(user *models.User, displayName, phone, source, lastLoginIP sql.NullString, lastLoginAt, passwordChangedAt sql.NullTime) {
	if displayName.Valid {
		user.DisplayName = displayName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if source.Valid {
		user.Source = source.String
	}
	if user.Source == "" {
		user.Source = "local"
	}
	if lastLoginIP.Valid {
		user.LastLoginIP = lastLoginIP.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if passwordChangedAt.Valid {
		user.PasswordChangedAt = &passwordChangedAt.Time
	}
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

func (r *UserRepository) CountByRole(ctx context.Context, role string) (int, error) {
	if r.db == nil || r.db.Pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	var count int
	err := r.db.Pool.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE role = ?`, role).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role: %w", err)
	}
	return count, nil
}
