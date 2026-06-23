package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/license"
)

type LicenseService struct {
	repo      *repositories.LicenseRepository
	auditSvc  *AuditService
	secret    string
}

func NewLicenseService(repo *repositories.LicenseRepository, auditSvc *AuditService, secret string) *LicenseService {
	return &LicenseService{repo: repo, auditSvc: auditSvc, secret: secret}
}

func (s *LicenseService) CurrentLicense(ctx context.Context) (*license.LicenseData, error) {
	l, err := s.repo.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, nil
	}
	return license.Parse([]byte(l.LicenseJSON), s.secret)
}

func (s *LicenseService) CurrentTier(ctx context.Context) license.Tier {
	l, err := s.CurrentLicense(ctx)
	if err != nil || l == nil {
		return license.TierCE
	}
	if !l.IsValid() {
		return license.TierCE
	}
	return l.Tier
}

func (s *LicenseService) HasFeature(ctx context.Context, f license.Feature) bool {
	l, err := s.CurrentLicense(ctx)
	if err != nil || l == nil || !l.IsValid() {
		return license.HasFeature(license.TierCE, f)
	}
	return license.HasFeature(l.Tier, f)
}

func (s *LicenseService) UploadLicense(ctx context.Context, jsonData []byte, operator string) (*license.LicenseData, error) {
	l, err := license.Parse(jsonData, s.secret)
	if err != nil {
		return nil, fmt.Errorf("invalid license: %w", err)
	}

	jsonStr := string(jsonData)
	dbLicense := &models.License{
		Tier:        string(l.Tier),
		LicenseKey:  l.LicenseKey,
		IssuedTo:    l.IssuedTo,
		IssuedAt:    l.IssuedAt,
		ExpiresAt:   l.ExpiresAt,
		MaxNodes:    l.MaxNodes,
		LicenseJSON: jsonStr,
		Active:      true,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.DeactivateAll(ctx); err != nil {
		return nil, fmt.Errorf("failed to deactivate old license: %w", err)
	}
	if err := s.repo.Save(ctx, dbLicense); err != nil {
		return nil, fmt.Errorf("failed to save license: %w", err)
	}

	return l, nil
}

func (s *LicenseService) LicenseInfo(ctx context.Context) (map[string]interface{}, error) {
	l, err := s.repo.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	if l == nil {
		tier := license.TierCE
		return map[string]interface{}{
			"tier":         string(tier),
			"license_key":  "",
			"issued_to":    "Community Edition",
			"issued_at":    nil,
			"expires_at":   nil,
			"max_nodes":    5,
			"active":       false,
			"features":     license.FeaturesForTier(tier),
		}, nil
	}

	parsed, err := license.Parse([]byte(l.LicenseJSON), s.secret)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tier":         l.Tier,
		"license_key":  l.LicenseKey,
		"issued_to":    l.IssuedTo,
		"issued_at":    l.IssuedAt,
		"expires_at":   l.ExpiresAt,
		"max_nodes":    l.MaxNodes,
		"active":       l.Active && parsed.IsValid(),
		"features":     parsed.Features(),
		"is_valid":     parsed.IsValid(),
		"is_expired":   parsed.IsExpired(),
	}, nil
}

func (s *LicenseService) Features(ctx context.Context) []license.Feature {
	return license.FeaturesForTier(s.CurrentTier(ctx))
}

func (s *LicenseService) GenerateLicense(ctx context.Context, tier license.Tier, issuedTo string, duration time.Duration, maxNodes int) (*license.LicenseData, error) {
	l := license.Generate(tier, issuedTo, duration, maxNodes, s.secret)
	jsonData, _ := json.Marshal(l)
	return l, s.repo.Save(ctx, &models.License{
		Tier:        string(tier),
		LicenseKey:  l.LicenseKey,
		IssuedTo:    issuedTo,
		IssuedAt:    l.IssuedAt,
		ExpiresAt:   l.ExpiresAt,
		MaxNodes:    maxNodes,
		LicenseJSON: string(jsonData),
		Active:      true,
		CreatedAt:   time.Now(),
	})
}
