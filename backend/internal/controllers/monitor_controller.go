package controllers

import (
	"net/http"
	"time"

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
	end := time.Now()
	start := end.Add(-1 * time.Hour)

	metrics, err := c.service.QueryMetrics(ctx.Request.Context(), services.MetricQueryRequest{
		InstanceID: instanceID,
		Metrics:    []string{},
		StartTime:  start,
		EndTime:    end,
	})
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to query metrics", err)
		return
	}

	status := c.service.CollectionStatus()
	message := ""
	if status == "not_configured" {
		message = "monitoring storage is not configured"
	} else if len(metrics) == 0 {
		status = "no_data"
		message = "no metrics have been ingested for this instance"
	}

	utils.SuccessResponse(ctx, gin.H{
		"status":  status,
		"message": message,
		"metrics": metrics,
	})
}

func (c *MonitorController) IngestMetrics(ctx *gin.Context) {
	var req services.MetricIngestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid metrics ingest request")
		return
	}
	if err := c.service.IngestMetrics(ctx.Request.Context(), req); err != nil {
		utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "Failed to ingest metrics", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"status": "ok"})
}
