package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type MonitorController struct {
	service *services.MonitorService
}

func NewMonitorController(service *services.MonitorService) *MonitorController {
	return &MonitorController{service: service}
}

func (c *MonitorController) QueryMetrics(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	
	metrics, err := c.service.QueryMetrics(ctx.Request.Context(), services.MetricQueryRequest{
		InstanceID: instanceID,
		Metrics:    []string{"qps", "tps", "connections"},
	})
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to query metrics", err)
		return
	}

	utils.SuccessResponse(ctx, metrics)
}