package repositories

import (
	"context"
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
	user.ID = uuid.New().String()
	
	query := `
		INSERT INTO users (id, username, password, email, role, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		user.ID, user.Username, user.Password, user.Email, user.Role, user.Status, user.CreatedAt)
	return err
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE username = $1
	`
	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.db == nil || r.db.Pool == nil {
		return nil, nil
	}
	query := `
		SELECT id, username, password, email, role, status, created_at, updated_at
		FROM users WHERE id = $1
	`
	user := &models.User{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}