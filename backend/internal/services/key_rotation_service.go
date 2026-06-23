package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type KeyRotationService struct {
	keyRepo  *repositories.KeyVersionRepository
	auditSvc *AuditService
}

func NewKeyRotationService(keyRepo *repositories.KeyVersionRepository, auditSvc *AuditService) *KeyRotationService {
	return &KeyRotationService{keyRepo: keyRepo, auditSvc: auditSvc}
}

func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (s *KeyRotationService) RotateKey(ctx context.Context, oldKey, newKey, note, operator string) (int, error) {
	if newKey == "" {
		return 0, fmt.Errorf("new key must not be empty")
	}
	if len(newKey) < 32 {
		return 0, fmt.Errorf("new key must be at least 32 characters")
	}

	if oldKey != "" {
		maxVer, err := s.keyRepo.GetMaxVersion(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get max version: %w", err)
		}
		kv := &models.KeyVersion{
			KeyDigest: keyDigest(oldKey),
			Version:   maxVer + 1,
			CreatedAt: time.Now(),
			Note:      "archived before rotation",
		}
		if maxVer == 0 {
			kv.Version = 1
			kv.Note = "initial key archived at first rotation"
		}
		if err := s.keyRepo.Create(ctx, kv); err != nil {
			return 0, fmt.Errorf("failed to archive old key: %w", err)
		}
	}

	records, err := s.keyRepo.GetAllEncryptedData(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list encrypted data: %w", err)
	}

	reEncrypted := 0
	for _, rec := range records {
		plain, decErr := utils.Decrypt(rec.Value, oldKey)
		if decErr != nil {
			return reEncrypted, fmt.Errorf("decryption failed for %s.%s row %s: %w", rec.Table, rec.ColumnName, rec.RowID, decErr)
		}
		newEnc, encErr := utils.Encrypt(plain, newKey)
		if encErr != nil {
			return reEncrypted, fmt.Errorf("encryption failed for %s.%s row %s: %w", rec.Table, rec.ColumnName, rec.RowID, encErr)
		}
		if err := s.keyRepo.UpdateEncryptedValue(ctx, rec.Table, rec.RowID, rec.ColumnName, newEnc); err != nil {
			return reEncrypted, err
		}
		reEncrypted++
	}

	return reEncrypted, nil
}

func (s *KeyRotationService) ListKeyVersions(ctx context.Context) ([]models.KeyVersion, error) {
	return s.keyRepo.List(ctx)
}
