package controllers

import (
	"github.com/gin-gonic/gin"
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