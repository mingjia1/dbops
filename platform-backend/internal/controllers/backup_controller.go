package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	policyID, err := c.service.CreatePolicy(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create backup policy", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"policy_id": policyID})
}

func (c *BackupController) ListPolicies(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")

	policies, err := c.service.ListPolicies(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list backup policies", err)
		return
	}

	utils.SuccessResponse(ctx, policies)
}

func (c *BackupController) ExecuteBackup(ctx *gin.Context) {
	var req services.ExecuteBackupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ExecuteBackup(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute backup", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) ListBackups(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")

	backups, err := c.service.ListBackups(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list backups", err)
		return
	}

	utils.SuccessResponse(ctx, backups)
}

func (c *BackupController) ScanBackups(ctx *gin.Context) {
	var req struct {
		InstanceID string `json:"instance_id" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ScanBackups(ctx.Request.Context(), req.InstanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to scan backups", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) RestoreBackup(ctx *gin.Context) {
	var req services.RestoreBackupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.RestoreBackup(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to restore backup", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *BackupController) DeleteBackupRecord(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.service.DeleteBackupRecord(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to delete backup record", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Backup record deleted successfully"})
}
