package controllers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type UpgradeController struct {
	service  *services.UpgradeService
	taskRepo *repositories.TaskRepository
}

func NewUpgradeController(service *services.UpgradeService, taskRepo *repositories.TaskRepository) *UpgradeController {
	return &UpgradeController{
		service:  service,
		taskRepo: taskRepo,
	}
}

func (c *UpgradeController) PlanUpgradePath(ctx *gin.Context) {
	var req services.PlanUpgradePathRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.PlanUpgradePath(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to plan upgrade path", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) CheckCompatibility(ctx *gin.Context) {
	var req services.CheckCompatibilityRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.CheckCompatibility(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to check compatibility", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteInPlaceUpgrade(ctx *gin.Context) {
	var req services.ExecuteInPlaceUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ExecuteInPlaceUpgrade(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute in-place upgrade", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteLogicalMigration(ctx *gin.Context) {
	var req services.ExecuteLogicalMigrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ExecuteLogicalMigration(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute logical migration", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteRollingUpgrade(ctx *gin.Context) {
	var req services.ExecuteRollingUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ExecuteRollingUpgrade(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute rolling upgrade", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) RollbackUpgrade(ctx *gin.Context) {
	var req services.RollbackUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.RollbackUpgrade(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to rollback upgrade", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ListHistory(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)
	tasks, err := c.taskRepo.ListByTypes(ctx.Request.Context(), []string{
		"upgrade_in_place",
		"upgrade_logical",
		"upgrade_rolling",
		"upgrade_rollback",
	}, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list upgrade history", err)
		return
	}
	history := make([]UpgradeHistoryItem, 0, len(tasks))
	for _, task := range tasks {
		history = append(history, mapUpgradeHistoryItem(task))
	}
	utils.SuccessResponse(ctx, history)
}

func (c *UpgradeController) GetUpgradeByID(ctx *gin.Context) {
	id := ctx.Param("id")

	task, err := c.taskRepo.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Upgrade task not found")
		return
	}

	utils.SuccessResponse(ctx, task)
}

func (c *UpgradeController) GenerateUpgradeReport(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.GenerateUpgradeReportRequest
	req.PlanID = id
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.ReportType = "full"
	}

	result, err := c.service.GenerateUpgradeReport(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to generate upgrade report", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

type UpgradeHistoryItem struct {
	ID          string    `json:"id"`
	TaskType    string    `json:"task_type"`
	UpgradeType string    `json:"upgrade_type"`
	PlanID      string    `json:"plan_id,omitempty"`
	InstanceID  string    `json:"instance_id"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	Stage       string    `json:"stage,omitempty"`
	Message     string    `json:"message,omitempty"`
	StartTime   time.Time `json:"start_time"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

func mapUpgradeHistoryItem(task models.Task) UpgradeHistoryItem {
	startTime := task.StartedAt
	if startTime.IsZero() {
		startTime = task.CreatedAt
	}
	message := task.ErrorMessage
	planID := ""
	if strings.HasPrefix(message, "plan:") {
		planID = strings.TrimPrefix(message, "plan:")
		message = ""
	}
	return UpgradeHistoryItem{
		ID:          task.ID,
		TaskType:    task.TaskType,
		UpgradeType: strings.TrimPrefix(task.TaskType, "upgrade_"),
		PlanID:      planID,
		InstanceID:  task.InstanceID,
		Status:      task.Status,
		Progress:    task.Progress,
		Stage:       inferUpgradeStage(task),
		Message:     message,
		StartTime:   startTime,
		CreatedAt:   task.CreatedAt,
		CompletedAt: task.CompletedAt,
	}
}

func inferUpgradeStage(task models.Task) string {
	switch task.Status {
	case "pending":
		return "queued"
	case "running":
		return "executing"
	case "completed", "success":
		return "completed"
	case "failed":
		return "failed"
	default:
		return task.Status
	}
}
