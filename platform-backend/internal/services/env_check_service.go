package services

import (
	"context"
	"fmt"
	"time"
)

type EnvironmentCheckService struct {
}

func NewEnvironmentCheckService() *EnvironmentCheckService {
	return &EnvironmentCheckService{}
}

type EnvironmentCheckRequest struct {
	Hosts []HostConfig `json:"hosts" binding:"required"`
}

type HostConfig struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type EnvironmentCheckResult struct {
	CheckID   string        `json:"check_id"`
	Status    string        `json:"status"`
	Results   []CheckResult `json:"results"`
	CreatedAt time.Time     `json:"created_at"`
}

type CheckResult struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Passed      bool   `json:"passed"`
	Value       string `json:"value"`
	Suggestion  string `json:"suggestion"`
}

func (s *EnvironmentCheckService) Execute(ctx context.Context, req EnvironmentCheckRequest) (*EnvironmentCheckResult, error) {
	result := &EnvironmentCheckResult{
		CheckID:   fmt.Sprintf("check-%d", time.Now().Unix()),
		Status:    "completed",
		CreatedAt: time.Now(),
		Results:   []CheckResult{},
	}

	for _, host := range req.Hosts {
		results := s.checkHost(host)
		result.Results = append(result.Results, results...)
	}

	return result, nil
}

func (s *EnvironmentCheckService) checkHost(host HostConfig) []CheckResult {
	return []CheckResult{
		{
			Category:   "hardware",
			Name:       "cpu_cores",
			Status:     "passed",
			Passed:     true,
			Value:      "8",
			Suggestion: "",
		},
		{
			Category:   "hardware",
			Name:       "memory_size",
			Status:     "passed",
			Passed:     true,
			Value:      "16GB",
			Suggestion: "",
		},
		{
			Category:   "hardware",
			Name:       "disk_space",
			Status:     "passed",
			Passed:     true,
			Value:      "100GB",
			Suggestion: "",
		},
		{
			Category:   "os",
			Name:       "kernel_version",
			Status:     "passed",
			Passed:     true,
			Value:      "5.4.0",
			Suggestion: "",
		},
		{
			Category:   "network",
			Name:       "port_3306",
			Status:     "passed",
			Passed:     true,
			Value:      "available",
			Suggestion: "",
		},
		{
			Category:   "dependency",
			Name:       "libaio",
			Status:     "passed",
			Passed:     true,
			Value:      "installed",
			Suggestion: "",
		},
	}
}

func (s *EnvironmentCheckService) GetByID(ctx context.Context, checkID string) (*EnvironmentCheckResult, error) {
	return &EnvironmentCheckResult{
		CheckID:   checkID,
		Status:    "completed",
		CreatedAt: time.Now(),
		Results:   []CheckResult{},
	}, nil
}