package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

type DrillService struct {
	drillRepo  *repositories.DrillRepository
	reportRepo *repositories.DrillReportRepository
	execRepo   *repositories.FaultExecutionRepository
}

func NewDrillService(drillRepo *repositories.DrillRepository, reportRepo *repositories.DrillReportRepository, execRepo *repositories.FaultExecutionRepository) *DrillService {
	return &DrillService{drillRepo: drillRepo, reportRepo: reportRepo, execRepo: execRepo}
}

// --- Drills ---

func (s *DrillService) Create(ctx context.Context, d *models.HADrill) error {
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	return s.drillRepo.Create(ctx, d)
}

func (s *DrillService) List(ctx context.Context, status string) ([]models.HADrill, error) {
	return s.drillRepo.List(ctx, status)
}

func (s *DrillService) Get(ctx context.Context, id string) (*models.HADrill, error) {
	return s.drillRepo.GetByID(ctx, id)
}

func (s *DrillService) Update(ctx context.Context, d *models.HADrill) error {
	d.UpdatedAt = time.Now()
	return s.drillRepo.Update(ctx, d)
}

func (s *DrillService) Delete(ctx context.Context, id string) error {
	return s.drillRepo.Delete(ctx, id)
}

// StartDrill changes status to "running".
func (s *DrillService) StartDrill(ctx context.Context, id string) (*models.HADrill, error) {
	d, err := s.drillRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	d.Status = "running"
	d.StartedAt = &now
	d.UpdatedAt = now
	if err := s.drillRepo.Update(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// CompleteDrill changes status to "completed" and generates a report.
func (s *DrillService) CompleteDrill(ctx context.Context, id string) (*models.HADrill, *models.HADrillReport, error) {
	d, err := s.drillRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	d.Status = "completed"
	d.CompletedAt = &now
	d.UpdatedAt = now

	// Calculate score from fault executions
	execs, _ := s.execRepo.List(ctx, id)
	total := len(execs)
	success := 0
	for _, e := range execs {
		if e.Status == "active" || e.Status == "rolled_back" {
			success++
		}
	}
	score := 0
	if total > 0 {
		score = success * 100 / total
	}
	d.Score = score
	d.Result = fmt.Sprintf("Drill completed: %d/%d fault injections handled successfully", success, total)

	if err := s.drillRepo.Update(ctx, d); err != nil {
		return nil, nil, err
	}

	// Generate report
	report := &models.HADrillReport{
		DrillID:     id,
		Summary:     d.Result,
		Timeline:    s.buildTimeline(execs),
		Findings:    s.buildFindings(execs),
		Score:       score,
		GeneratedAt: now,
		CreatedAt:   now,
	}
	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, nil, err
	}

	return d, report, nil
}

func (s *DrillService) buildTimeline(execs []models.FaultExecution) string {
	type entry struct {
		Time   string `json:"time"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	entries := make([]entry, 0, len(execs))
	for _, e := range execs {
		entries = append(entries, entry{
			Time:   e.StartedAt.Format(time.RFC3339),
			Type:   e.FaultType,
			Status: e.Status,
		})
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

func (s *DrillService) buildFindings(execs []models.FaultExecution) string {
	type finding struct {
		FaultType string `json:"fault_type"`
		Target    string `json:"target"`
		Result    string `json:"result"`
		Recovered bool   `json:"recovered"`
	}
	findings := make([]finding, 0, len(execs))
	for _, e := range execs {
		findings = append(findings, finding{
			FaultType: e.FaultType,
			Target:    e.TargetID,
			Result:    e.Result,
			Recovered: e.RollbackAt != nil,
		})
	}
	b, _ := json.Marshal(findings)
	return string(b)
}

func (s *DrillService) GetReport(ctx context.Context, drillID string) (*models.HADrillReport, error) {
	return s.reportRepo.GetByDrillID(ctx, drillID)
}
