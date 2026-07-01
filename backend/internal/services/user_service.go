package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo *repositories.UserRepository
	roleRepo *repositories.RoleRepository
	auditSvc *AuditService
}

func NewUserService(userRepo *repositories.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) SetRoleRepository(roleRepo *repositories.RoleRepository) {
	s.roleRepo = roleRepo
}

func (s *UserService) SetAuditService(auditSvc *AuditService) {
	s.auditSvc = auditSvc
}

type CreateUserRequest struct {
	Username    string   `json:"username" binding:"required"`
	Password    string   `json:"password" binding:"required,min=6"`
	Email       string   `json:"email" binding:"required,email"`
	Role        string   `json:"role" binding:"required"`
	Roles       []string `json:"roles"`
	Status      string   `json:"status"`
	DisplayName string   `json:"display_name"`
	Phone       string   `json:"phone"`
}

type UpdateUserRequest struct {
	Username    string   `json:"username" binding:"required"`
	Password    string   `json:"password" binding:"omitempty,min=6"`
	Email       string   `json:"email" binding:"required,email"`
	Role        string   `json:"role" binding:"required"`
	Roles       []string `json:"roles"`
	Status      string   `json:"status" binding:"required"`
	DisplayName string   `json:"display_name"`
	Phone       string   `json:"phone"`
}

type ResetUserPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type UpdateUserRolesRequest struct {
	Roles []string `json:"roles" binding:"required"`
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
		s.attachRoles(ctx, &users[i])
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
	s.attachRoles(ctx, user)
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
		Username:    req.Username,
		Password:    string(hash),
		Email:       req.Email,
		Role:        req.Role,
		Status:      status,
		DisplayName: req.DisplayName,
		Phone:       req.Phone,
		Source:      "local",
		CreatedAt:   time.Now(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	roles := normalizedRoles(req.Role, req.Roles)
	if s.roleRepo != nil {
		if err := s.roleRepo.SetUserRolesByName(ctx, user.ID, roles); err != nil {
			return nil, err
		}
	}
	s.auditUser(ctx, "create_user", "create", user.ID, "success", "username="+user.Username)
	s.attachRoles(ctx, user)
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
	nextRoles := normalizedRoles(req.Role, req.Roles)
	if user.Role == "admin" && !containsString(nextRoles, "admin") {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, errors.New("cannot remove the last admin role")
		}
	}
	user.Username = req.Username
	user.Email = req.Email
	user.Role = nextRoles[0]
	user.Status = req.Status
	user.DisplayName = req.DisplayName
	user.Phone = req.Phone
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
	if s.roleRepo != nil {
		if err := s.roleRepo.SetUserRolesByName(ctx, user.ID, nextRoles); err != nil {
			return nil, err
		}
	}
	s.auditUser(ctx, "update_user", "update", user.ID, "success", "username="+user.Username)
	s.attachRoles(ctx, user)
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
	if err := s.userRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.auditUser(ctx, "delete_user", "delete", id, "success", "username="+user.Username)
	return nil
}

func (s *UserService) Enable(ctx context.Context, id string) error {
	return s.setStatus(ctx, id, "active")
}

func (s *UserService) Disable(ctx context.Context, id string) error {
	return s.setStatus(ctx, id, "disabled")
}

func (s *UserService) ResetPassword(ctx context.Context, id string, req ResetUserPasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, id, string(hash)); err != nil {
		return err
	}
	s.auditUser(ctx, "reset_user_password", "reset_password", id, "success", "username="+user.Username)
	return nil
}

func (s *UserService) UpdateRoles(ctx context.Context, id string, req UpdateUserRolesRequest) (*models.User, error) {
	if s.roleRepo == nil {
		return nil, errors.New("role repository not available")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	roles := normalizedRoles(user.Role, req.Roles)
	if user.Role == "admin" && !containsString(roles, "admin") {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, errors.New("cannot remove the last admin role")
		}
	}
	user.Role = roles[0]
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	if err := s.roleRepo.SetUserRolesByName(ctx, id, roles); err != nil {
		return nil, err
	}
	s.attachRoles(ctx, user)
	user.Password = ""
	s.auditUser(ctx, "update_user_roles", "update_roles", id, "success", "roles="+fmt.Sprint(roles))
	return user, nil
}

func (s *UserService) setStatus(ctx context.Context, id, status string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if user.Role == "admin" && status != "active" {
		adminCount, err := s.userRepo.CountByRole(ctx, "admin")
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("cannot disable the last admin user")
		}
	}
	if err := s.userRepo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	s.auditUser(ctx, status+"_user", "update_status", id, "success", "status="+status)
	return nil
}

func (s *UserService) attachRoles(ctx context.Context, user *models.User) {
	if s.roleRepo == nil || user == nil {
		return
	}
	roles, err := s.roleRepo.ListByUserID(ctx, user.ID)
	if err == nil && len(roles) > 0 {
		user.Roles = roles
		user.Permissions = mergeRolePermissions(roles)
		return
	}
	if role, err := s.roleRepo.GetByName(ctx, user.Role); err == nil && role != nil {
		user.Roles = []models.Role{*role}
		user.Permissions = role.Permissions
	}
}

func (s *UserService) auditUser(ctx context.Context, operation, action, resourceID, result, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: "user",
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
	})
}

func normalizedRoles(primary string, roles []string) []string {
	if len(roles) == 0 {
		if primary == "" {
			return []string{"operator"}
		}
		return []string{primary}
	}
	out := make([]string, 0, len(roles))
	seen := map[string]bool{}
	for _, role := range roles {
		if role != "" && !seen[role] {
			out = append(out, role)
			seen[role] = true
		}
	}
	if len(out) == 0 {
		return normalizedRoles(primary, nil)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
