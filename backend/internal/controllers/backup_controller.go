package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type BackupController struct {
	service *services.BackupService
}

func NewBackupController(service *services.BackupService) *BackupController {
	return &BackupController{service: service}
}

func (c *BackupController) CreatePolicy(ctx *gin.Context) {
	var req services.CreateBackupPolicyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	policyID, err := c.service.CreatePolicy(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建备份策略失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"policy_id": policyID})
}

func (c *BackupController) ListPolicies(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")

	policies, err := c.service.ListPolicies(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取备份策略列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, policies)
}

func (c *BackupController) UpdatePolicy(ctx *gin.Context) {
	id := ctx.Param("id")
	var req services.UpdateBackupPolicyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	policy, err := c.service.UpdatePolicy(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "更新备份策略失败", err)
		return
	}

	utils.SuccessResponse(ctx, policy)
}

func (c *BackupController) DeletePolicy(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeletePolicy(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除备份策略失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Backup policy deleted successfully"})
}

func (c *BackupController) ExecuteBackup(ctx *gin.Context) {
	var req services.ExecuteBackupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ExecuteBackup(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, err.Error(), err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) ListBackups(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")

	backups, err := c.service.ListBackups(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取备份列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, backups)
}

func (c *BackupController) ScanBackups(ctx *gin.Context) {
	var req struct {
		InstanceID string `json:"instance_id" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ScanBackups(ctx.Request.Context(), req.InstanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "扫描备份失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) RestoreBackup(ctx *gin.Context) {
	var req services.RestoreBackupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.RestoreBackup(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "恢复备份失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) DeleteBackupRecord(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteBackupRecord(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除备份记录失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Backup record deleted successfully"})
}
