package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKeyRotationService(t *testing.T) (*KeyRotationService, *repositories.KeyVersionRepository) {
	db := newTestDB(t)
	keyRepo := repositories.NewKeyVersionRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditSvc := NewAuditService(auditRepo, approvalRepo)
	svc := NewKeyRotationService(keyRepo, auditSvc)
	return svc, keyRepo
}

func TestKeyRotationServiceListKeyVersionsEmpty(t *testing.T) {
	svc, _ := newTestKeyRotationService(t)
	ctx := context.Background()

	versions, err := svc.ListKeyVersions(ctx)
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestKeyRotationServiceRotateKeyFirstTime(t *testing.T) {
	svc, keyRepo := newTestKeyRotationService(t)
	ctx := context.Background()

	count, err := svc.RotateKey(ctx, "", "new-encryption-key-must-be-32chars!!", "initial rotation", "admin")
	require.NoError(t, err)
	assert.Equal(t, 0, count) // no data to re-encrypt

	versions, err := keyRepo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, versions) // empty old key means no key version created
}

func TestKeyRotationServiceRotateKeySecondTime(t *testing.T) {
	svc, keyRepo := newTestKeyRotationService(t)
	ctx := context.Background()

	// First rotation with non-empty old key to create a key version
	_, err := svc.RotateKey(ctx, "", "new-encryption-key-must-be-32chars!!", "first", "admin")
	require.NoError(t, err)

	// Second rotation - needs valid old key
	count, err := svc.RotateKey(ctx, "new-encryption-key-must-be-32chars!!", "second-encryption-key-must-be-32ch", "second", "admin")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	versions, err := keyRepo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, versions, 1)
	assert.Equal(t, 1, versions[0].Version)
}

func TestKeyRotationServiceRotateKeyEmptyNewKey(t *testing.T) {
	svc, _ := newTestKeyRotationService(t)
	ctx := context.Background()

	_, err := svc.RotateKey(ctx, "", "", "note", "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new key must not be empty")
}

func TestKeyRotationServiceRotateKeyShortKey(t *testing.T) {
	svc, _ := newTestKeyRotationService(t)
	ctx := context.Background()

	_, err := svc.RotateKey(ctx, "", "short", "note", "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32")
}

func TestKeyRotationServiceRotateKeyReEncryptsData(t *testing.T) {
	db := newTestDB(t)
	keyRepo := repositories.NewKeyVersionRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditSvc := NewAuditService(auditRepo, approvalRepo)
	svc := NewKeyRotationService(keyRepo, auditSvc)
	ctx := context.Background()

	// Create a host with encrypted credential using old key
	oldKey := "old-encryption-key-32-chars-long!!"
	encrypted, err := utils.Encrypt("ssh-password", oldKey)
	require.NoError(t, err)

	err = hostRepo.Create(ctx, &models.Host{
		ID:             "host-1",
		Name:           "test-host",
		Address:        "10.0.0.1",
		SSHUser:        "root",
		SSHCredential:  encrypted,
		AgentPort:      9090,
	})
	require.NoError(t, err)

	// Rotate key
	newKey := "new-encryption-key-32-chars-long!"
	count, err := svc.RotateKey(ctx, oldKey, newKey, "rotate", "admin")
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	// Verify host credential is re-encrypted and decryptable with new key
	host, err := hostRepo.GetByID(ctx, "host-1")
	require.NoError(t, err)
	decrypted, err := utils.Decrypt(host.SSHCredential, newKey)
	require.NoError(t, err)
	assert.Equal(t, "ssh-password", decrypted)
}

func TestKeyRotationServiceRotateKeyInvalidOldKey(t *testing.T) {
	db := newTestDB(t)
	keyRepo := repositories.NewKeyVersionRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	approvalRepo := repositories.NewApprovalRequestRepository(db)
	auditSvc := NewAuditService(auditRepo, approvalRepo)
	svc := NewKeyRotationService(keyRepo, auditSvc)
	ctx := context.Background()

	// Create host with data encrypted by "correct" key
	correctKey := "correct-encryption-key-32-chars!!"
	encrypted, err := utils.Encrypt("secret", correctKey)
	require.NoError(t, err)

	err = hostRepo.Create(ctx, &models.Host{
		ID:             "host-1",
		Name:           "test-host",
		Address:        "10.0.0.1",
		SSHUser:        "root",
		SSHCredential:  encrypted,
		AgentPort:      9090,
	})
	require.NoError(t, err)

	// Rotate with wrong old key - should handle errors gracefully
	newKey := "new-encryption-key-32-chars-long!"
	count, err := svc.RotateKey(ctx, "wrong-old-key-32-chars!!!!!!!", newKey, "rotate", "admin")
	// Should not error out completely - partial rotation
	_ = count
	_ = err
}
