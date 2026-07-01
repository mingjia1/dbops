package services

import (
	"context"
	"errors"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

type RoleService struct {
	roleRepo *repositories.RoleRepository
	auditSvc *AuditService
}

func NewRoleService(roleRepo *repositories.RoleRepository) *RoleService {
	return &RoleService{roleRepo: roleRepo}
}

func (s *RoleService) SetAuditService(auditSvc *AuditService) {
	s.auditSvc = auditSvc
}

type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions" binding:"required"`
}

type UpdateRoleRequest struct {
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions" binding:"required"`
}

func (s *RoleService) List(ctx context.Context) ([]models.Role, error) {
	if s.roleRepo == nil {
		return nil, errors.New("role repository not available")
	}
	return s.roleRepo.List(ctx)
}

func (s *RoleService) Create(ctx context.Context, req CreateRoleRequest) (*models.Role, error) {
	if s.roleRepo == nil {
		return nil, errors.New("role repository not available")
	}
	if existing, err := s.roleRepo.GetByName(ctx, req.Name); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, errors.New("role already exists")
	}
	role := &models.Role{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Permissions: req.Permissions,
	}
	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, err
	}
	s.auditRole(ctx, "create_role", "create", role.ID, "success")
	return role, nil
}

func (s *RoleService) Update(ctx context.Context, id string, req UpdateRoleRequest) (*models.Role, error) {
	if s.roleRepo == nil {
		return nil, errors.New("role repository not available")
	}
	role := &models.Role{
		ID:          id,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Permissions: req.Permissions,
	}
	if err := s.roleRepo.Update(ctx, role); err != nil {
		return nil, err
	}
	s.auditRole(ctx, "update_role", "update", id, "success")
	return role, nil
}

func (s *RoleService) Delete(ctx context.Context, id string) error {
	if s.roleRepo == nil {
		return errors.New("role repository not available")
	}
	if err := s.roleRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.auditRole(ctx, "delete_role", "delete", id, "success")
	return nil
}

func (s *RoleService) auditRole(ctx context.Context, operation, action, resourceID, result string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: "role",
		ResourceID:   resourceID,
		Action:       action,
		Result:       result,
	})
}
