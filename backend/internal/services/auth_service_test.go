package services

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func createAuthTestUser(t *testing.T, ctx context.Context, repo *repositories.UserRepository, id, username, password, role string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &models.User{
		ID:        id,
		Username:  username,
		Password:  string(hash),
		Email:     username + "@localhost",
		Role:      role,
		Status:    "active",
		CreatedAt: time.Now(),
	}))
}

func newAuthAuditTestService() (*AuthService, *repositories.UserRepository, *repositories.AuditLogRepository) {
	db := newTestDB(t)
	userRepo := repositories.NewUserRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	return NewAuthService(userRepo, "test-jwt-secret", auditSvc), userRepo, auditRepo
}

func TestAuthChangePasswordWritesAuditLogAndUpdatesPassword(t *testing.T) {
	service, userRepo, auditRepo := newAuthAuditTestService()
	ctx := context.WithValue(context.Background(), "user_id", "user-001")
	createAuthTestUser(t, ctx, userRepo, "user-001", "alice", "old-password", "dba")

	err := service.ChangePassword(ctx, "user-001", ChangePasswordRequest{
		CurrentPassword: "old-password",
		NewPassword:     "new-password",
	})

	require.NoError(t, err)
	_, err = service.Login(context.Background(), LoginRequest{Username: "alice", Password: "new-password"})
	require.NoError(t, err)

	logs, err := auditRepo.ListByResource(context.Background(), "user", "user-001", 10, 0)
	require.NoError(t, err)
	changeLog := findAuditLogByOperation(logs, "change_password")
	require.NotNil(t, changeLog)
	assert.Equal(t, "user-001", changeLog.UserID)
	assert.Equal(t, "change_password", changeLog.Operation)
	assert.Equal(t, "change_password", changeLog.Action)
	assert.Equal(t, "success", changeLog.Result)
	assert.NotContains(t, changeLog.Details, "new-password")
	assert.NotContains(t, changeLog.Details, "old-password")
}

func TestAuthResetAllPasswordsWritesAuditLogAndUpdatesUsers(t *testing.T) {
	service, userRepo, auditRepo := newAuthAuditTestService()
	ctx := context.WithValue(context.Background(), "user_id", "admin-001")
	createAuthTestUser(t, ctx, userRepo, "user-001", "alice", "old-password-1", "dba")
	createAuthTestUser(t, ctx, userRepo, "user-002", "bob", "old-password-2", "operator")

	updated, err := service.ResetAllPasswords(ctx, ResetAllPasswordsRequest{NewPassword: "123456"})

	require.NoError(t, err)
	assert.Equal(t, int64(2), updated)
	_, err = service.Login(context.Background(), LoginRequest{Username: "alice", Password: "123456"})
	require.NoError(t, err)
	_, err = service.Login(context.Background(), LoginRequest{Username: "bob", Password: "123456"})
	require.NoError(t, err)

	logs, err := auditRepo.ListByResource(context.Background(), "user", "all", 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "admin-001", logs[0].UserID)
	assert.Equal(t, "reset_all_passwords", logs[0].Operation)
	assert.Equal(t, "reset_password", logs[0].Action)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "updated_count=2")
	assert.NotContains(t, logs[0].Details, "123456")
}

func TestAuthValidateTokenRejectsDisabledUser(t *testing.T) {
	service, userRepo, _ := newAuthAuditTestService()
	ctx := context.Background()
	createAuthTestUser(t, ctx, userRepo, "user-003", "carol", "old-password", "operator")

	resp, err := service.Login(ctx, LoginRequest{Username: "carol", Password: "old-password"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Token)

	require.NoError(t, userRepo.UpdateStatus(ctx, "user-003", "disabled"))
	claims, err := service.ValidateToken(resp.Token)

	require.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "not active")
}

func TestAuthLoginReturnsPermissions(t *testing.T) {
	service, userRepo, _ := newAuthAuditTestService()
	ctx := context.Background()
	createAuthTestUser(t, ctx, userRepo, "user-004", "dana", "old-password", "admin")

	resp, err := service.Login(ctx, LoginRequest{Username: "dana", Password: "old-password"})

	require.NoError(t, err)
	require.Contains(t, resp.User.Permissions, "*")
}

func TestUserServiceCannotRemoveLastAdminViaUpdate(t *testing.T) {
	db := newTestDB(t)
	userRepo := repositories.NewUserRepository(db)
	roleRepo := repositories.NewRoleRepository(db)
	require.NoError(t, roleRepo.SeedBuiltinRoles(context.Background()))
	service := NewUserService(userRepo)
	service.SetRoleRepository(roleRepo)
	ctx := context.Background()
	createAuthTestUser(t, ctx, userRepo, "admin-002", "root", "old-password", "admin")

	_, err := service.Update(ctx, "admin-002", UpdateUserRequest{
		Username: "root",
		Email:    "root@localhost",
		Role:     "operator",
		Roles:    []string{"operator"},
		Status:   "active",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "last admin")
}

func TestUserRepositoryUpdatePasswordReturnsNotFoundForMissingUser(t *testing.T) {
	repo := repositories.NewUserRepository(newTestDB(t))

	err := repo.UpdatePassword(context.Background(), "missing-user", "hash")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func findAuditLogByOperation(logs []models.AuditLog, operation string) *models.AuditLog {
	for i := range logs {
		if logs[i].Operation == operation {
			return &logs[i]
		}
	}
	return nil
}
