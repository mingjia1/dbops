package controllers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type ClusterDeployController struct {
	service *services.ClusterDeployService
}

func NewClusterDeployController(service *services.ClusterDeployService) *ClusterDeployController {
	return &ClusterDeployController{service: service}
}

func (c *ClusterDeployController) DeployMHA(ctx *gin.Context) {
	var req services.DeployMHARequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.DeployMHA(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy MHA cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployMGR(ctx *gin.Context) {
	var req services.DeployMGRRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.DeployMGR(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy MGR cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployPXC(ctx *gin.Context) {
	var req services.DeployPXCRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.DeployPXC(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy PXC cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployHA(ctx *gin.Context) {
	var req services.DeployHARequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.DeployHA(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy HA master-replica cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) GetDeploymentStatus(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.GetDeploymentStatus(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Deployment not found")
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) List(ctx *gin.Context) {
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	offset, err := strconv.Atoi(ctx.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	response, err := c.service.ListDeployments(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list cluster deployments", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) Destroy(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.DestroyCluster(ctx.Request.Context(), deploymentID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "Deployment not found")
			return
		}
		var destroyErr *services.ClusterDestroyOperationError
		if errors.As(err, &destroyErr) {
			utils.ErrorResponse(ctx, 409, err.Error(), err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "Failed to destroy cluster deployment", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}
