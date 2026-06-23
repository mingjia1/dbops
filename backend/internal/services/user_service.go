package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo *repositories.UserRepository
}

func NewUserService(userRepo *repositories.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required"`
	Status   string `json:"status"`
}

type UpdateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"omitempty,min=6"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required"`
	Status   string `json:"status" binding:"required"`
}

func (s *UserService) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not available")
	}
	users, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].Password = ""
	}
	return users, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not available")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	user.Password = ""
	return user, nil
}

func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*models.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not available")
	}
	if existing, err := s.userRepo.GetByUsername(ctx, req.Username); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, errors.New("username already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	user := &models.User{
		Username:  req.Username,
		Password:  string(hash),
		Email:     req.Email,
		Role:      req.Role,
		Status:    status,
		CreatedAt: time.Now(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}

func (s *UserService) Update(ctx context.Context, id string, req UpdateUserRequest) (*models.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not available")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if req.Username != user.Username {
		if existing, err := s.userRepo.GetByUsername(ctx, req.Username); err != nil {
			return nil, err
		} else if existing != nil && existing.ID != id {
			return nil, errors.New("username already exists")
		}
	}
	user.Username = req.Username
	user.Email = req.Email
	user.Role = req.Role
	user.Status = req.Status
	user.UpdatedAt = time.Now()
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		user.Password = string(hash)
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	if s.userRepo == nil {
		return errors.New("user repository not available")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if user.Role == "admin" {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return fmt.Errorf("failed to count admin users: %w", err)
		}
		if adminCount <= 1 {
			return errors.New("cannot delete the last admin user")
		}
	}
	return s.userRepo.Delete(ctx, id)
}
