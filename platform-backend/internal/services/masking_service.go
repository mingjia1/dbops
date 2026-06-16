package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type MaskingService struct {
	repo *repositories.MaskingRepository
}

func NewMaskingService(repo *repositories.MaskingRepository) *MaskingService {
	return &MaskingService{repo: repo}
}

func (s *MaskingService) Create(ctx context.Context, rule *models.MaskingRule) error {
	return s.repo.Create(ctx, rule)
}

func (s *MaskingService) GetByID(ctx context.Context, id string) (*models.MaskingRule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MaskingService) List(ctx context.Context) ([]models.MaskingRule, error) {
	return s.repo.List(ctx)
}

func (s *MaskingService) Update(ctx context.Context, rule *models.MaskingRule) error {
	return s.repo.Update(ctx, rule)
}

func (s *MaskingService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *MaskingService) MaskField(value string, rule *models.MaskingRule) string {
	switch rule.Algorithm {
	case models.MaskingMD5:
		return maskMD5(value)
	case models.MaskingMask:
		return maskWithPattern(value, rule.Pattern, rule.Replacement)
	case models.MaskingReplace:
		if rule.Replacement != "" {
			return rule.Replacement
		}
		return "***"
	default:
		return "***"
	}
}

func (s *MaskingService) GetEnabledRules(ctx context.Context) ([]models.MaskingRule, error) {
	return s.repo.ListEnabled(ctx)
}

func maskMD5(value string) string {
	if value == "" {
		return ""
	}
	return "***"
}

func maskWithPattern(value, pattern, replacement string) string {
	if pattern == "" {
		if len(value) <= 4 {
			return strings.Repeat("*", len(value))
		}
		return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return value
	}
	rep := replacement
	if rep == "" {
		rep = "***"
	}
	return re.ReplaceAllString(value, rep)
}
