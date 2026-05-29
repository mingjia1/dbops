package controllers

import (
	"net/http"
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