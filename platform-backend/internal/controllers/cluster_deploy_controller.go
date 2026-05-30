package controllers

import (
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

func (c *ClusterDeployController) GetDeploymentStatus(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.GetDeploymentStatus(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Deployment not found")
		return
	}

	utils.SuccessResponse(ctx, response)
}