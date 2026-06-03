package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
)

type UserRepository struct {
	db *Database
}

func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	if r.db == nil || r.db.Pool == nil {
		return nil
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
		return nil, nil
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE username = ?
	`
	user := &models.User{}
	var updatedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role, &user.Status, &user.CreatedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE id = ?
	`
	user := &models.User{}
	var updatedAt sql.NullTime
	err := r.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role, &user.Status, &user.CreatedAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time
	}
	return user, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return []models.User{}, nil
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
		var updatedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &updatedAt); err != nil {
			return nil, err
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
		return nil
	}
	query := `
		UPDATE users SET username = ?, password = ?, email = ?, role = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Pool.ExecContext(ctx, query,
		user.Username, user.Password, user.Email, user.Role, user.Status, user.UpdatedAt, user.ID)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if r.db == nil || r.db.Pool == nil {
		return nil
	}
	_, err := r.db.Pool.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}
