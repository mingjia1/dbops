package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type TopologyController struct {
	service *services.TopologyService
}

func NewTopologyController(service *services.TopologyService) *TopologyController {
	return &TopologyController{service: service}
}

func (c *TopologyController) GetInstanceTopology(ctx *gin.Context) {
	instanceID := ctx.Param("id")

	topology, err := c.service.GetInstanceTopology(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Instance topology not found")
		return
	}

	utils.SuccessResponse(ctx, topology)
}

func (c *TopologyController) GetClusterTopology(ctx *gin.Context) {
	clusterID := ctx.Param("cluster_id")

	topology, err := c.service.GetClusterTopology(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.NotFoundResponse(ctx, "Cluster topology not found")
		return
	}

	utils.SuccessResponse(ctx, topology)
}

func (c *TopologyController) GetTopologyGraph(ctx *gin.Context) {
	clusterID := ctx.Param("cluster_id")

	graph, err := c.service.BuildTopologyGraph(ctx.Request.Context(), clusterID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to build topology graph", err)
		return
	}

	utils.SuccessResponse(ctx, graph)
}
