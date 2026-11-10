package controllers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
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
		utils.InternalServerErrorResponse(ctx, "获取迁移任务列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, tasks)
}

func (c *MigrationController) GetByID(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "缺少任务 ID")
		return
	}

	task, err := c.service.GetTask(ctx.Request.Context(), taskID)
	if err != nil {
		utils.NotFoundResponse(ctx, "迁移任务不存在")
		return
	}

	utils.SuccessResponse(ctx, task)
}

func (c *MigrationController) ExecutePhysical(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建迁移任务失败", err)
		return
	}

	result, err := c.service.ExecutePhysicalMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行物理迁移失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) ExecuteReplication(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建迁移任务失败", err)
		return
	}

	result, err := c.service.ExecuteReplicationMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行复制迁移失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) ExecuteGTID(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建迁移任务失败", err)
		return
	}

	result, err := c.service.ExecuteGTIDMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行 GTID 迁移失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) Orchestrate(ctx *gin.Context) {
	var req services.CreateMigrationTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	taskID, err := c.service.CreateTask(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建迁移任务失败", err)
		return
	}

	result, err := c.service.OrchestrateMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "迁移编排失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) GetProgress(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "缺少任务 ID")
		return
	}

	progress, err := c.service.MonitorMigrationProgress(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取迁移进度失败", err)
		return
	}

	utils.SuccessResponse(ctx, progress)
}

func (c *MigrationController) Verify(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "缺少任务 ID")
		return
	}

	verification, err := c.service.VerifyMigration(ctx.Request.Context(), taskID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "迁移校验失败", err)
		return
	}

	utils.SuccessResponse(ctx, verification)
}

func (c *MigrationController) Switch(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "缺少任务 ID")
		return
	}

	result, err := c.service.ExecuteSwitch(ctx.Request.Context(), taskID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "迁移任务不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行切换失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *MigrationController) Cancel(ctx *gin.Context) {
	taskID := ctx.Param("id")
	if taskID == "" {
		utils.BadRequestResponse(ctx, "缺少任务 ID")
		return
	}

	if err := c.service.CancelTask(ctx.Request.Context(), taskID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "迁移任务不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "取消迁移任务失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"task_id": taskID, "status": "cancelled"})
}
