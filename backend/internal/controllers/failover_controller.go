package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type FailoverController struct {
	service *services.FailoverService
}

func NewFailoverController(service *services.FailoverService) *FailoverController {
	return &FailoverController{service: service}
}

func (c *FailoverController) ExecuteAutoFailover(ctx *gin.Context) {
	var req services.FailoverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	if err := c.service.ValidateFailoverRequest(ctx.Request.Context(), req); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	result, err := c.service.ExecuteAutoFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行自动故障切换失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) ExecuteManualFailover(ctx *gin.Context) {
	var req services.FailoverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	if err := c.service.ValidateFailoverRequest(ctx.Request.Context(), req); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	result, err := c.service.ExecuteManualFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行手动故障切换失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) PreflightFailover(ctx *gin.Context) {
	var req services.FailoverPreflightRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.PreflightFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "执行预检失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) GetClusterStatus(ctx *gin.Context) {
	clusterID := ctx.Query("cluster_id")
	if clusterID == "" {
		utils.BadRequestResponse(ctx, "缺少 cluster_id")
		return
	}

	master, err := c.service.GetCurrentMaster(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取当前主节点失败", err)
		return
	}

	slaves, err := c.service.GetSlaves(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取从节点列表失败", err)
		return
	}

	historyLimit := 10
	limitStr := ctx.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err == nil && limit > 0 {
		historyLimit = limit
	}

	history, err := c.service.GetFailoverHistory(ctx.Request.Context(), clusterID, historyLimit)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取故障切换历史失败", err)
		return
	}

	status := gin.H{
		"cluster_id": clusterID,
		"master":     master,
		"slaves":     slaves,
		"history":    history,
	}

	utils.SuccessResponse(ctx, status)
}
