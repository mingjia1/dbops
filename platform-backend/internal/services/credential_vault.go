package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type CredentialVault struct {
	repo  *repositories.CredentialRepository
	encKey string
}

func NewCredentialVault(repo *repositories.CredentialRepository, encKey string) *CredentialVault {
	return &CredentialVault{repo: repo, encKey: encKey}
}

func (v *CredentialVault) GetCredential(ctx context.Context, clusterID, accountType string) (*models.ClusterCredential, error) {
	return v.repo.GetByClusterAndType(ctx, clusterID, accountType)
}

func (v *CredentialVault) GetDecryptedPassword(ctx context.Context, clusterID, accountType string) (string, error) {
	cred, err := v.repo.GetByClusterAndType(ctx, clusterID, accountType)
	if err != nil {
		return "", err
	}
	plaintext, err := utils.Decrypt(cred.PasswordEnc, v.encKey)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}
	return plaintext, nil
}

func (v *CredentialVault) SetCredential(ctx context.Context, clusterID, accountType, username, password string) error {
	encrypted, err := utils.Encrypt(password, v.encKey)
	if err != nil {
		return fmt.Errorf("encrypt credential: %w", err)
	}
	cred := &models.ClusterCredential{
		ID:          uuid.New().String(),
		ClusterID:   clusterID,
		AccountType: accountType,
		Username:    username,
		PasswordEnc: encrypted,
		CreatedAt:   time.Now(),
	}
	return v.repo.Create(ctx, cred)
}

func (v *CredentialVault) RotateClusterCredentials(ctx context.Context, clusterID string) (map[string]string, error) {
	creds, err := v.repo.ListByCluster(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}

	newPasswords := make(map[string]string)
	for _, cred := range creds {
		newPass, err := GenerateSecurePassword(24)
		if err != nil {
			return nil, fmt.Errorf("generate password: %w", err)
		}
		encrypted, err := utils.Encrypt(newPass, v.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt: %w", err)
		}
		if err := v.repo.UpdatePassword(ctx, cred.ID, encrypted); err != nil {
			return nil, fmt.Errorf("update password: %w", err)
		}
		newPasswords[cred.AccountType] = newPass
	}
	return newPasswords, nil
}

func (v *CredentialVault) ListClusterCredentials(ctx context.Context, clusterID string) ([]models.ClusterCredential, error) {
	return v.repo.ListByCluster(ctx, clusterID)
}

func (v *CredentialVault) DeleteClusterCredentials(ctx context.Context, clusterID string) error {
	return v.repo.DeleteByCluster(ctx, clusterID)
}

func GenerateSecurePassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}
