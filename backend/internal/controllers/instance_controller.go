package controllers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type InstanceController struct {
	service *services.InstanceService
}

func NewInstanceController(service *services.InstanceService) *InstanceController {
	return &InstanceController{service: service}
}

func (c *InstanceController) Create(ctx *gin.Context) {
	var req services.CreateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	instance, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) BatchCreate(ctx *gin.Context) {
	var req services.BatchCreateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.BatchCreate(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "批量创建实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	instance, err := c.service.GetByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "实例不存在")
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)
	hostID := ctx.Query("host_id")

	var instances []models.Instance
	var err error
	if hostID != "" {
		instances, err = c.service.ListByHostID(ctx.Request.Context(), hostID, limit, offset)
	} else {
		instances, err = c.service.List(ctx.Request.Context(), limit, offset)
	}
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取实例列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, instances)
}

func (c *InstanceController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateInstanceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	instance, err := c.service.Update(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "更新实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Instance deleted successfully"})
}

func (c *InstanceController) DetectVersion(ctx *gin.Context) {
	id := ctx.Param("id")

	version, err := c.service.DetectVersion(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "版本探测失败", err)
		return
	}

	utils.SuccessResponse(ctx, version)
}

func (c *InstanceController) Deploy(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.Deploy(ctx.Request.Context(), id)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "部署实例失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) HealthCheck(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.HealthCheck(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "实例健康检查失败", err)
		return
	}

	if isFailedInstanceTaskStatus(result.Status) {
		ctx.JSON(200, utils.Response{
			Code:      200,
			Message:   result.Message,
			Data:      result,
			Timestamp: time.Now(),
			TraceID:   ctx.GetString("trace_id"),
		})
		return
	}

	utils.SuccessResponse(ctx, result)
}

func isFailedInstanceTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "unhealthy", "timeout", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (c *InstanceController) GetReplicationStatus(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.service.ReplicationStatus(ctx.Request.Context(), id)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取复制状态失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) AdminAction(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.InstanceAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.AdminAction(ctx.Request.Context(), id, req)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行实例管理操作失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) ForceResetPassword(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.ForceResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	if err := c.service.ForceResetInstancePassword(ctx.Request.Context(), id, req); err != nil {
		utils.InternalServerErrorResponse(ctx, "强制重置密码失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"message": "Password reset successfully"})
}

func (c *InstanceController) UpdateStatus(ctx *gin.Context) {
	id := ctx.Param("id")

	var req services.UpdateInstanceStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	instance, err := c.service.UpdateInstanceStatus(ctx.Request.Context(), id, req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "更新实例状态失败", err)
		return
	}

	utils.SuccessResponse(ctx, instance)
}

func (c *InstanceController) RecoverCluster(ctx *gin.Context) {
	id := ctx.Param("id")
	result, err := c.service.RecoverCluster(ctx.Request.Context(), id)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "集群恢复失败", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *InstanceController) GetCredentials(ctx *gin.Context) {
	id := ctx.Param("id")
	adminUser := ctx.GetString("user_id")
	log.Printf("AUDIT: admin=%s accessed credentials for instance=%s", adminUser, id)

	creds, err := c.service.GetInstanceCredentials(ctx.Request.Context(), id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "实例凭据不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "获取实例凭据失败", err)
		return
	}

	utils.SuccessResponse(ctx, creds)
}

func (c *InstanceController) BatchUpdatePassword(ctx *gin.Context) {
	var req services.BatchPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.BatchUpdatePassword(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "更新实例密码失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}
