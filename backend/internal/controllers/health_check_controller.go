package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type HealthCheckController struct {
	service *services.HealthCheckService
}

func NewHealthCheckController(service *services.HealthCheckService) *HealthCheckController {
	return &HealthCheckController{service: service}
}

func (c *HealthCheckController) ExecuteHealthCheck(ctx *gin.Context) {
	var req services.HealthCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// GET requests don't have bodies; fall back to query params
		req.InstanceID = ctx.Query("instance_id")
	}
	if req.InstanceID == "" {
		req.InstanceID = ctx.Query("instance_id")
	}
	if req.InstanceID == "" {
		utils.BadRequestResponse(ctx, "缺少 instance_id")
		return
	}

	result, err := c.service.ExecuteHealthCheck(ctx.Request.Context(), req)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "执行健康检查失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) DetectFailure(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "缺少 instance_id")
		return
	}

	result, err := c.service.DetectFailure(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "故障检测失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) GetFailureState(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "缺少 instance_id")
		return
	}

	state := c.service.GetFailureState(instanceID)
	if state == nil {
		utils.NotFoundResponse(ctx, "故障状态不存在")
		return
	}

	utils.SuccessResponse(ctx, state)
}

func (c *HealthCheckController) BatchHealthCheck(ctx *gin.Context) {
	var req struct {
		InstanceIDs []string `json:"instance_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	results, err := c.service.BatchHealthCheck(ctx.Request.Context(), req.InstanceIDs)
	if err != nil {
				if services.IsAgentTransportError(err) {
			utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Agent 不在线或网络不可达，请检查主机 Agent 状态后重试", err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "批量执行健康检查失败", err)
		return
	}

	utils.SuccessResponse(ctx, results)
}
