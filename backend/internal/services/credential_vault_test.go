package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCredentialRepo() *repositories.CredentialRepository {
	return repositories.NewCredentialRepository(newTestDB())
}

func TestCredentialVault_SetAndGet(t *testing.T) {
	repo := newTestCredentialRepo()
	vault := NewCredentialVault(repo, "test-key-123456")
	ctx := context.Background()

	err := vault.SetCredential(ctx, "cluster-001", "root", "root", "secret123")
	require.NoError(t, err)

	cred, err := vault.GetCredential(ctx, "cluster-001", "root")
	require.NoError(t, err)
	assert.Equal(t, "root", cred.Username)
	assert.Equal(t, "cluster-001", cred.ClusterID)
	assert.Equal(t, "root", cred.AccountType)
	assert.NotEmpty(t, cred.PasswordEnc)
}

func TestCredentialVault_GetDecryptedPassword(t *testing.T) {
	repo := newTestCredentialRepo()
	encKey := "test-key-secure"
	vault := NewCredentialVault(repo, encKey)
	ctx := context.Background()

	err := vault.SetCredential(ctx, "cluster-001", "repl", "repl_user", "repl_pass_456")
	require.NoError(t, err)

	plain, err := vault.GetDecryptedPassword(ctx, "cluster-001", "repl")
	require.NoError(t, err)
	assert.Equal(t, "repl_pass_456", plain)
}

func TestCredentialVault_GetDecryptedPassword_WrongKey(t *testing.T) {
	repo := newTestCredentialRepo()
	vault1 := NewCredentialVault(repo, "key-a")
	ctx := context.Background()

	err := vault1.SetCredential(ctx, "cluster-001", "root", "root", "pass123")
	require.NoError(t, err)

	vault2 := NewCredentialVault(repo, "key-b")
	_, err = vault2.GetDecryptedPassword(ctx, "cluster-001", "root")
	assert.Error(t, err)
}

func TestCredentialVault_NotFound(t *testing.T) {
	repo := newTestCredentialRepo()
	vault := NewCredentialVault(repo, "test-key")
	ctx := context.Background()

	_, err := vault.GetCredential(ctx, "nonexistent", "root")
	assert.Error(t, err)
}

func TestCredentialVault_RotateClusterCredentials(t *testing.T) {
	repo := newTestCredentialRepo()
	encKey := "rotate-test-key"
	vault := NewCredentialVault(repo, encKey)
	ctx := context.Background()

	require.NoError(t, vault.SetCredential(ctx, "cluster-002", "root", "root", "old_root"))
	require.NoError(t, vault.SetCredential(ctx, "cluster-002", "repl", "repl", "old_repl"))

	newPasses, err := vault.RotateClusterCredentials(ctx, "cluster-002")
	require.NoError(t, err)
	assert.Len(t, newPasses, 2)
	assert.NotEmpty(t, newPasses["root"])
	assert.NotEmpty(t, newPasses["repl"])
	assert.NotEqual(t, "old_root", newPasses["root"])
	assert.NotEqual(t, "old_repl", newPasses["repl"])

	decryptedRoot, err := vault.GetDecryptedPassword(ctx, "cluster-002", "root")
	require.NoError(t, err)
	assert.Equal(t, newPasses["root"], decryptedRoot)
}

func TestCredentialVault_ListAndDelete(t *testing.T) {
	repo := newTestCredentialRepo()
	vault := NewCredentialVault(repo, "test-key")
	ctx := context.Background()

	require.NoError(t, vault.SetCredential(ctx, "cluster-003", "root", "root", "pass1"))
	require.NoError(t, vault.SetCredential(ctx, "cluster-003", "repl", "repl", "pass2"))
	require.NoError(t, vault.SetCredential(ctx, "cluster-003", "monitor", "monitor", "pass3"))

	creds, err := vault.ListClusterCredentials(ctx, "cluster-003")
	require.NoError(t, err)
	assert.Len(t, creds, 3)

	err = vault.DeleteClusterCredentials(ctx, "cluster-003")
	require.NoError(t, err)

	creds, err = vault.ListClusterCredentials(ctx, "cluster-003")
	require.NoError(t, err)
	assert.Len(t, creds, 0)
}

func TestGenerateSecurePassword(t *testing.T) {
	p1, err := GenerateSecurePassword(24)
	require.NoError(t, err)
	assert.Len(t, p1, 24)

	p2, err := GenerateSecurePassword(24)
	require.NoError(t, err)
	assert.NotEqual(t, p1, p2)
}

func TestCredentialVault_EncryptionRoundTrip(t *testing.T) {
	encKey := "roundtrip-key"
	original := "my_super_secret_password!@#"

	encrypted, err := utils.Encrypt(original, encKey)
	require.NoError(t, err)
	assert.NotEqual(t, original, encrypted)

	decrypted, err := utils.Decrypt(encrypted, encKey)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

func TestCredentialVault_SyncCredentialToNode(t *testing.T) {
	repo := newTestCredentialRepo()
	vault := NewCredentialVault(repo, "sync-test-key")
	ctx := context.Background()

	err := vault.SetCredential(ctx, "cluster-sync", "root", "root", "secret123")
	require.NoError(t, err)

	var callHost string
	var callPath string
	fakeAgent := func(_ context.Context, host string, _ int, path string, payload map[string]interface{}) (map[string]interface{}, error) {
		callHost = host
		callPath = path
		return map[string]interface{}{"status": "ok"}, nil
	}

	err = vault.SyncCredentialToNode(ctx, "cluster-sync", "root", "10.0.0.1", 9090, fakeAgent)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", callHost)
	assert.Equal(t, "/api/v1/accounts/rotate", callPath)
}

func TestCredentialVault_SyncCredentialToNode_NilCaller(t *testing.T) {
	repo := newTestCredentialRepo()
	vault := NewCredentialVault(repo, "sync-test-key")
	ctx := context.Background()

	err := vault.SyncCredentialToNode(ctx, "cluster-001", "root", "10.0.0.1", 9090, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}
