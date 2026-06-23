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
		INSERT INTO users (id, username, password, email, role, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		user.ID, user.Username, user.Password, user.Email, user.Role, user.Status, user.CreatedAt)
	return err
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE username = ?
	`
	user := &models.User{}
	var email sql.NullString
	var updatedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Password, &email, &user.Role, &user.Status, &user.CreatedAt, &updatedAt)
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
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE id = ?
	`
	user := &models.User{}
	var email sql.NullString
	var updatedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Password, &email, &user.Role, &user.Status, &user.CreatedAt, &updatedAt)
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
	return user, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
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
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &email, &u.Role, &u.Status, &u.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		if email.Valid {
			u.Email = email.String
		}
		if updatedAt.Valid {
			u.UpdatedAt = updatedAt.Time
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	query := `
		UPDATE users SET username = ?, password = ?, email = ?, role = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		user.Username, user.Password, user.Email, user.Role, user.Status, user.UpdatedAt, user.ID)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if r.db == nil || r.db.Pool == nil {
		return fmt.Errorf("database not available")
	}
	res, err := r.db.Pool.ExecContext(ctx, `UPDATE users SET password = ?, updated_at = ? WHERE id = ?`, passwordHash, time.Now(), userID)
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
