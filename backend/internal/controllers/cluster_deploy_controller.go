package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

func rejectPseudoUniversalMode(ctx *gin.Context, mode string) bool {
	if strings.EqualFold(strings.TrimSpace(mode), "pseudo") {
		utils.BadRequestResponse(ctx, "伪部署模式已移除：预览方案请使用 validate_only，真实部署请省略 mode")
		return true
	}
	return false
}

func rejectPseudoModeFromBody(ctx *gin.Context) bool {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return true
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	if enabled, ok := raw["pseudo_mode"].(bool); ok && enabled {
		utils.BadRequestResponse(ctx, "伪部署模式已移除，请提交真实部署请求")
		return true
	}
	return false
}

func (c *ClusterDeployController) DeployCluster(ctx *gin.Context) {
	var req services.UniversalClusterDeployRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	if rejectPseudoUniversalMode(ctx, req.Mode) {
		return
	}

	response, err := c.service.SubmitClusterDeploy(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "部署集群失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) ValidateClusterDeploy(ctx *gin.Context) {
	var req services.UniversalClusterDeployRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	if rejectPseudoUniversalMode(ctx, req.Mode) {
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
		ClusterType string                            `json:"cluster_type"`
		HostIDs     []string                          `json:"host_ids"`
		Nodes       []services.ClusterDeployCheckNode `json:"nodes"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "预检请求无效")
		return
	}
	results, err := c.service.PreCheck(ctx.Request.Context(), req.ClusterType, req.HostIDs, req.Nodes)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "预检失败", err)
		return
	}
	utils.SuccessResponse(ctx, results)
}

func (c *ClusterDeployController) RepairPreCheck(ctx *gin.Context) {
	var req services.ClusterDeployPrecheckRepairRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "预检修复请求无效")
		return
	}
	result, err := c.service.RepairPreCheck(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "预检修复失败", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) DeployMHA(ctx *gin.Context) {
	if rejectPseudoModeFromBody(ctx) {
		return
	}
	var req services.DeployMHARequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	universalReq := services.TypedMHARequestToUniversal(req)
	response, err := c.service.SubmitClusterDeploy(ctx.Request.Context(), universalReq)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "部署 MHA 集群失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployMGR(ctx *gin.Context) {
	if rejectPseudoModeFromBody(ctx) {
		return
	}
	var req services.DeployMGRRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	universalReq := services.TypedMGRRequestToUniversal(req)
	response, err := c.service.SubmitClusterDeploy(ctx.Request.Context(), universalReq)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "部署 MGR 集群失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployPXC(ctx *gin.Context) {
	if rejectPseudoModeFromBody(ctx) {
		return
	}
	var req services.DeployPXCRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	universalReq := services.TypedPXCRequestToUniversal(req)
	response, err := c.service.SubmitClusterDeploy(ctx.Request.Context(), universalReq)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "部署 PXC 集群失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) DeployHA(ctx *gin.Context) {
	if rejectPseudoModeFromBody(ctx) {
		return
	}
	var req services.DeployHARequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	universalReq := services.TypedHARequestToUniversal(req)
	response, err := c.service.SubmitClusterDeploy(ctx.Request.Context(), universalReq)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "部署 HA 主从集群失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) GetDeployPlan(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.GetDeployPlan(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "方案不存在")
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) GetDeploymentStatus(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.GetDeploymentStatus(ctx.Request.Context(), deploymentID)
	if err != nil {
		utils.NotFoundResponse(ctx, "部署记录不存在")
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
		utils.InternalServerErrorResponse(ctx, "获取集群部署列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) ChangeClusterPassword(ctx *gin.Context) {
	clusterID := ctx.Param("id")

	var req services.ClusterPasswordChangeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	result, err := c.service.ChangeClusterPassword(ctx.Request.Context(), clusterID, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "部署记录不存在")
			return
		}
		utils.InternalServerErrorResponse(ctx, "修改集群密码失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) Destroy(ctx *gin.Context) {
	deploymentID := ctx.Param("id")

	response, err := c.service.DestroyCluster(ctx.Request.Context(), deploymentID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.NotFoundResponse(ctx, "部署记录不存在")
			return
		}
		var destroyErr *services.ClusterDestroyOperationError
		if errors.As(err, &destroyErr) {
			utils.ErrorResponse(ctx, 409, err.Error(), err)
			return
		}
		utils.InternalServerErrorResponse(ctx, "销毁集群部署失败", err)
		return
	}

	utils.SuccessResponse(ctx, response)
}

func (c *ClusterDeployController) ScaleOut(ctx *gin.Context) {
	deploymentID := ctx.Param("id")
	var req struct {
		NodeCount int      `json:"node_count"`
		HostIDs   []string `json:"host_ids"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求无效")
		return
	}
	if len(req.HostIDs) == 0 {
		utils.BadRequestResponse(ctx, "缺少 host_ids")
		return
	}
	if req.NodeCount > 0 && req.NodeCount != len(req.HostIDs) {
		utils.BadRequestResponse(ctx, "node_count 必须与 host_ids 数量一致")
		return
	}

	result, err := c.service.ScaleOutCluster(ctx.Request.Context(), deploymentID, req.HostIDs)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "扩容失败", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) ScaleIn(ctx *gin.Context) {
	deploymentID := ctx.Param("id")
	var req struct {
		RemoveNodeID string `json:"remove_node_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求无效")
		return
	}

	if req.RemoveNodeID == "" {
		utils.BadRequestResponse(ctx, "缺少 remove_node_id")
		return
	}

	result, err := c.service.ScaleInCluster(ctx.Request.Context(), deploymentID, req.RemoveNodeID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "缩容失败", err)
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
		utils.BadRequestResponse(ctx, "请求无效")
		return
	}

	if req.NodeID == "" {
		utils.BadRequestResponse(ctx, "缺少 node_id")
		return
	}

	result, err := c.service.RebuildClusterNode(ctx.Request.Context(), deploymentID, req.NodeID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "重建失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *ClusterDeployController) ListClusters(ctx *gin.Context) {
	clusters, err := c.service.ListClusters(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取集群列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, clusters)
}

func (c *ClusterDeployController) GetClusterDetail(ctx *gin.Context) {
	clusterID := ctx.Param("cluster_id")
	detail, err := c.service.GetClusterDetail(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.NotFoundResponse(ctx, "集群不存在")
		return
	}
	utils.SuccessResponse(ctx, detail)
}
