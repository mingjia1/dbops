package controllers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type ClusterDeployController struct {
	service *services.ClusterDeployService
}

func NewClusterDeployController(service *services.ClusterDeployService) *ClusterDeployController {
	return &ClusterDeployController{service: service}
}

func (c *ClusterDeployController) DeployCluster(ctx *gin.Context) {
	var req services.UniversalClusterDeployRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.DeployCluster(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) ValidateClusterDeploy(ctx *gin.Context) {
	var req services.UniversalClusterDeployRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	response, err := c.service.ValidateClusterDeploy(ctx.Request.Context(), req)
	if err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) PreCheck(ctx *gin.Context) {
	var req struct {
		HostIDs []string                          `json:"host_ids"`
		Nodes   []services.ClusterDeployCheckNode `json:"nodes"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "invalid precheck request")
		return
	}
	results, err := c.service.PreCheck(ctx.Request.Context(), req.HostIDs, req.Nodes)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Pre-check failed", err)
		return
	}
	utils.SuccessResponse(ctx, results)
}

func (c *ClusterDeployController) DeployMHA(ctx *gin.Context) {
	var req services.DeployMHARequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	universalReq := services.TypedMHARequestToUniversal(req)
	response, err := c.service.DeployCluster(ctx.Request.Context(), universalReq)
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

	universalReq := services.TypedMGRRequestToUniversal(req)
	response, err := c.service.DeployCluster(ctx.Request.Context(), universalReq)
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

	universalReq := services.TypedPXCRequestToUniversal(req)
	response, err := c.service.DeployCluster(ctx.Request.Context(), universalReq)
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

	universalReq := services.TypedHARequestToUniversal(req)
	response, err := c.service.DeployCluster(ctx.Request.Context(), universalReq)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to deploy HA master-replica cluster", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) GetDeployPlan(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.GetDeployPlan(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Plan not found")
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

func (c *ClusterDeployController) ChangeClusterPassword(ctx *gin.Context) {
	clusterID := ctx.Param("id")

	var req services.ClusterPasswordChangeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	result, err := c.service.ChangeClusterPassword(ctx.Request.Context(), clusterID, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "Deployment not found")
			return
		}
		utils.InternalServerErrorResponse(ctx, "Failed to change cluster password", err)
		return
	}

	utils.SuccessResponse(ctx, result)
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

func (c *ClusterDeployController) ScaleOut(ctx *gin.Context) {
	deploymentID := ctx.Param("id")
	var req struct {
		NodeCount int `json:"node_count"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request")
		return
	}

	dep, err := c.service.GetDeploymentStatus(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Deployment not found")
		return
	}

	// Scale-out requires additional host selection via the deployment UI.
	// Return the deployment context so the frontend can prompt the user for new hosts.
	utils.SuccessResponse(ctx, gin.H{
		"deployment_id": deploymentID,
		"cluster_type":  dep.ClusterType,
		"cluster_id":    dep.ClusterID,
		"action":        "scale-out",
		"node_count":    req.NodeCount,
		"status":        "awaiting_hosts",
		"message":       "Please select hosts for the new nodes via the deployment page",
	})
}

func (c *ClusterDeployController) ScaleIn(ctx *gin.Context) {
	deploymentID := ctx.Param("id")
	var req struct {
		RemoveNodeID string `json:"remove_node_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request")
		return
	}

	if req.RemoveNodeID == "" {
		utils.BadRequestResponse(ctx, "remove_node_id is required")
		return
	}

	result, err := c.service.ScaleInCluster(ctx.Request.Context(), deploymentID, req.RemoveNodeID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Scale-in failed", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) RebuildNode(ctx *gin.Context) {
	deploymentID := ctx.Param("id")
	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request")
		return
	}

	if req.NodeID == "" {
		utils.BadRequestResponse(ctx, "node_id is required")
		return
	}

	result, err := c.service.RebuildClusterNode(ctx.Request.Context(), deploymentID, req.NodeID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Rebuild failed", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) ListClusters(ctx *gin.Context) {
	clusters, err := c.service.ListClusters(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list clusters", err)
		return
	}
	utils.SuccessResponse(ctx, clusters)
}

func (c *ClusterDeployController) GetClusterDetail(ctx *gin.Context) {
	clusterID := ctx.Param("cluster_id")
	detail, err := c.service.GetClusterDetail(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Cluster not found")
		return
	}
	utils.SuccessResponse(ctx, detail)
}
