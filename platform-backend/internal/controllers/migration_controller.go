package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type MigrationController struct {
	service *services.MigrationService
}

func NewMigrationController(service *services.MigrationService) *MigrationController {
	return &MigrationController{service: service}
}

func (c *MigrationController) List(ctx *gin.Context) {
	tasks, err := c.service.ListTasks(ctx.Request.Context(), "")
	if err != nil {
		utils.SuccessResponse(ctx, []interface{}{})
		return
	}
	utils.SuccessResponse(ctx, tasks)
}

func (c *MigrationController) GetByID(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "Task ID is required")
		return
	}

	task, err := c.service.GetTask(ctx.Request.Context(), taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Migration task not found")
		return
	}

	utils.SuccessResponse(ctx, task)
}

func (c *MigrationController) ExecutePhysical(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create migration task", err)
		return
	}

	result, err := c.service.ExecutePhysicalMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute physical migration", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) ExecuteReplication(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create migration task", err)
		return
	}

	result, err := c.service.ExecuteReplicationMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute replication migration", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) ExecuteGTID(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create migration task", err)
		return
	}

	result, err := c.service.ExecuteGTIDMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute GTID migration", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) Orchestrate(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create migration task", err)
		return
	}

	result, err := c.service.OrchestrateMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to orchestrate migration", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) GetProgress(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "Task ID is required")
		return
	}

	progress, err := c.service.MonitorMigrationProgress(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to get migration progress", err)
		return
	}

	utils.SuccessResponse(ctx, progress)
}

func (c *MigrationController) Verify(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "Task ID is required")
		return
	}

	verification, err := c.service.VerifyMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to verify migration", err)
		return
	}

	utils.SuccessResponse(ctx, verification)
}

func (c *MigrationController) Switch(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "Task ID is required")
		return
	}

	result, err := c.service.ExecuteSwitch(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute switch", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) Cancel(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "Task ID is required")
		return
	}

	if err := c.service.CancelTask(ctx.Request.Context(), taskID); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to cancel migration task", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"task_id": taskID, "status": "cancelled"})
}
