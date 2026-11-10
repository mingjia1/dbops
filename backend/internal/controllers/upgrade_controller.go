package controllers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
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
	var controllerReq struct {
		InstanceID    string `json:"instance_id" binding:"required"`
		TargetVersion string `json:"target_version" binding:"required"`
		TargetFlavor  string `json:"target_flavor"`
		Strategy      string `json:"strategy" binding:"required"`
		SourceVersion string `json:"source_version"`
		FromVersion   string `json:"from_version"`
	}
	if err := ctx.ShouldBindJSON(&controllerReq); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.PlanUpgradePath(ctx.Request.Context(), services.PlanUpgradePathRequest{
		InstanceID:    controllerReq.InstanceID,
		TargetVersion: controllerReq.TargetVersion,
		TargetFlavor:  controllerReq.TargetFlavor,
		Strategy:      controllerReq.Strategy,
		SourceVersion: controllerReq.SourceVersion,
		FromVersion:   controllerReq.FromVersion,
	})
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "规划升级路径失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) CheckCompatibility(ctx *gin.Context) {
	var req services.CheckCompatibilityRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.CheckCompatibility(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "兼容性检查失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteInPlaceUpgrade(ctx *gin.Context) {
	var req services.ExecuteInPlaceUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ExecuteInPlaceUpgrade(ctx.Request.Context(), req)
	if err != nil {
		if ue, ok := utils.IsUpgradeError(err); ok {
			utils.ErrorResponse(ctx, utils.UpgradeHTTPStatus(ue), ue.Message, nil)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行原地升级失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteLogicalMigration(ctx *gin.Context) {
	var req services.ExecuteLogicalMigrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ExecuteLogicalMigration(ctx.Request.Context(), req)
	if err != nil {
		if ue, ok := utils.IsUpgradeError(err); ok {
			utils.ErrorResponse(ctx, utils.UpgradeHTTPStatus(ue), ue.Message, nil)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行逻辑迁移失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) ExecuteRollingUpgrade(ctx *gin.Context) {
	var req services.ExecuteRollingUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ExecuteRollingUpgrade(ctx.Request.Context(), req)
	if err != nil {
		if ue, ok := utils.IsUpgradeError(err); ok {
			utils.ErrorResponse(ctx, utils.UpgradeHTTPStatus(ue), ue.Message, nil)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行滚动升级失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *UpgradeController) RollbackUpgrade(ctx *gin.Context) {
	var req services.RollbackUpgradeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.RollbackUpgrade(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "回滚升级失败", err)
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
		utils.InternalServerErrorResponse(ctx, "获取升级历史失败", err)
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
		utils.NotFoundResponse(ctx, "升级任务不存在")
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
		utils.InternalServerErrorResponse(ctx, "生成升级报告失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

type UpgradeHistoryItem struct {
	ID            string    `json:"id"`
	TaskType      string    `json:"task_type"`
	UpgradeType   string    `json:"upgrade_type"`
	PlanID        string    `json:"plan_id,omitempty"`
	SourceVersion string    `json:"source_version,omitempty"`
	TargetVersion string    `json:"target_version,omitempty"`
	InstanceID    string    `json:"instance_id"`
	Status        string    `json:"status"`
	Progress      int       `json:"progress"`
	Stage         string    `json:"stage,omitempty"`
	Message       string    `json:"message,omitempty"`
	StartTime     time.Time `json:"start_time"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

func mapUpgradeHistoryItem(task models.Task) UpgradeHistoryItem {
	startTime := task.StartedAt
	if startTime.IsZero() {
		startTime = task.CreatedAt
	}
	planID, sourceVersion, targetVersion, message := parseUpgradeTaskMeta(task.ErrorMessage)
	return UpgradeHistoryItem{
		ID:            task.ID,
		TaskType:      task.TaskType,
		UpgradeType:   strings.TrimPrefix(task.TaskType, "upgrade_"),
		PlanID:        planID,
		SourceVersion: sourceVersion,
		TargetVersion: targetVersion,
		InstanceID:    task.InstanceID,
		Status:        task.Status,
		Progress:      task.Progress,
		Stage:         inferUpgradeStage(task),
		Message:       message,
		StartTime:     startTime,
		CreatedAt:     task.CreatedAt,
		CompletedAt:   task.CompletedAt,
	}
}

// parseUpgradeTaskMeta extracts planID, sourceVersion, targetVersion, and message
// from the task's ErrorMessage field. Format: "plan:<id>|source:<ver>|target:<ver>\n<message>"
func parseUpgradeTaskMeta(raw string) (planID, sourceVersion, targetVersion, message string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	metaAndMsg := strings.SplitN(raw, "\n", 2)
	meta := metaAndMsg[0]
	if len(metaAndMsg) > 1 {
		message = strings.TrimSpace(metaAndMsg[1])
	}
	for _, part := range strings.Split(meta, "|") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "plan:"):
			planID = strings.TrimPrefix(part, "plan:")
		case strings.HasPrefix(part, "source:"):
			sourceVersion = strings.TrimPrefix(part, "source:")
		case strings.HasPrefix(part, "target:"):
			targetVersion = strings.TrimPrefix(part, "target:")
		}
	}
	return
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
