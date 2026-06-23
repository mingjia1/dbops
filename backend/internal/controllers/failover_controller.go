package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if err := c.service.ValidateFailoverRequest(ctx.Request.Context(), req); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	result, err := c.service.ExecuteAutoFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute auto failover", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) ExecuteManualFailover(ctx *gin.Context) {
	var req services.FailoverRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if err := c.service.ValidateFailoverRequest(ctx.Request.Context(), req); err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	result, err := c.service.ExecuteManualFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute manual failover", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) PreflightFailover(ctx *gin.Context) {
	var req services.FailoverPreflightRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.PreflightFailover(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute preflight check", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *FailoverController) GetClusterStatus(ctx *gin.Context) {
	clusterID := ctx.Query("cluster_id")
	if clusterID == "" {
		utils.BadRequestResponse(ctx, "cluster_id is required")
		return
	}

	master, err := c.service.GetCurrentMaster(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to get current master", err)
		return
	}

	slaves, err := c.service.GetSlaves(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to get slaves", err)
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
		utils.InternalServerErrorResponse(ctx, "Failed to get failover history", err)
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
