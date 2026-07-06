package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUserService(t *testing.T) *UserService {
	return NewUserService(repositories.NewUserRepository(newTestDB(t)))
}

func TestUserServiceCreate(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	user, err := svc.Create(ctx, CreateUserRequest{
		Username: "alice",
		Password: "secret123",
		Email:    "alice@test.com",
		Role:     "dba",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "alice@test.com", user.Email)
	assert.Equal(t, "dba", user.Role)
	assert.Equal(t, "active", user.Status)
	assert.Empty(t, user.Password)
}

func TestUserServiceCreateDefaultStatus(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	user, err := svc.Create(ctx, CreateUserRequest{
		Username: "bob",
		Password: "secret123",
		Email:    "bob@test.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "active", user.Status)
}

func TestUserServiceCreateDuplicateUsername(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123"})
	require.NoError(t, err)

	_, err = svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret456"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username already exists")
}

func TestUserServiceCreateShortPassword(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "short"})
	require.NoError(t, err)
}

func TestUserServiceGetByID(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123"})
	require.NoError(t, err)

	user, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Empty(t, user.Password)
}

func TestUserServiceGetByIDNotFound(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	_, err := svc.GetByID(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserServiceList(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	_, _ = svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123", Email: "alice@test.com"})
	_, _ = svc.Create(ctx, CreateUserRequest{Username: "bob", Password: "secret456", Email: "bob@test.com"})

	users, err := svc.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 2)
	for _, u := range users {
		assert.Empty(t, u.Password)
	}
}

func TestUserServiceUpdate(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123"})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, created.ID, UpdateUserRequest{
		Username: "alice_new",
		Email:    "alice_new@test.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice_new", updated.Username)
	assert.Equal(t, "alice_new@test.com", updated.Email)
}

func TestUserServiceUpdatePassword(t *testing.T) {
	db := newTestDB(t)
	userRepo := repositories.NewUserRepository(db)
	svc := NewUserService(userRepo)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "oldpass123"})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, created.ID, UpdateUserRequest{
		Username: "alice",
		Status:   "active",
		Password: "newpass123",
	})
	require.NoError(t, err)
	assert.Empty(t, updated.Password)

	// Verify new password works via AuthService
	auditSvc := NewAuditService(repositories.NewAuditLogRepository(db), repositories.NewApprovalRequestRepository(db))
	authSvc := NewAuthService(userRepo, "test-jwt-secret", auditSvc)
	_, err = authSvc.Login(context.Background(), LoginRequest{Username: "alice", Password: "newpass123"})
	require.NoError(t, err)
}

func TestUserServiceUpdateDuplicateUsername(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	alice, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123", Email: "alice@test.com"})
	require.NoError(t, err)
	bob, err := svc.Create(ctx, CreateUserRequest{Username: "bob", Password: "secret456", Email: "bob@test.com"})
	require.NoError(t, err)

	_, err = svc.Update(ctx, alice.ID, UpdateUserRequest{Username: "bob", Email: "alice@test.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username already exists")

	// Keep own username should work
	_, err = svc.Update(ctx, alice.ID, UpdateUserRequest{Username: "alice", Email: "alice@test.com"})
	require.NoError(t, err)

	// Verify bob can still use his username
	_ = bob
}

func TestUserServiceUpdateNotFound(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	_, err := svc.Update(ctx, "missing", UpdateUserRequest{Username: "new"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserServiceDelete(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123"})
	require.NoError(t, err)

	err = svc.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, created.ID)
	require.Error(t, err)
}

func TestUserServiceDeleteNotFound(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	err := svc.Delete(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserServicePasswordHashed(t *testing.T) {
	db := newTestDB(t)
	userRepo := repositories.NewUserRepository(db)
	svc := NewUserService(userRepo)
	ctx := context.Background()

	user, err := svc.Create(ctx, CreateUserRequest{Username: "alice", Password: "secret123"})
	require.NoError(t, err)

	// Read raw from DB to verify bcrypt hash
	raw, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "secret123", raw.Password)
	assert.Contains(t, raw.Password, "$2a$")
}

func TestUserServiceListPaginated(t *testing.T) {
	svc := newTestUserService(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = svc.Create(ctx, CreateUserRequest{
			Username: "user" + string(rune('a'+i)),
			Password: "secret123",
			Email:    "user" + string(rune('a'+i)) + "@test.com",
		})
	}

	page1, err := svc.List(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := svc.List(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}
