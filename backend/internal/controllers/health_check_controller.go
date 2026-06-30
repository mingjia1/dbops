package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type HealthCheckController struct {
	service         *services.HealthCheckService
	instanceService *services.InstanceService
}

func NewHealthCheckController(service *services.HealthCheckService, instanceService ...*services.InstanceService) *HealthCheckController {
	ctrl := &HealthCheckController{service: service}
	if len(instanceService) > 0 {
		ctrl.instanceService = instanceService[0]
	}
	return ctrl
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
		utils.BadRequestResponse(ctx, "instance_id is required")
		return
	}

	result, err := c.service.ExecuteHealthCheck(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute health check", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) DetectFailure(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "instance_id is required")
		return
	}

	result, err := c.service.DetectFailure(ctx.Request.Context(), instanceID)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to detect failure", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *HealthCheckController) GetFailureState(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
	if instanceID == "" {
		utils.BadRequestResponse(ctx, "instance_id is required")
		return
	}

	state := c.service.GetFailureState(instanceID)
	if state == nil {
		utils.NotFoundResponse(ctx, "Failure state not found")
		return
	}

	utils.SuccessResponse(ctx, state)
}

func (c *HealthCheckController) BatchHealthCheck(ctx *gin.Context) {
	var req struct {
		InstanceIDs []string `json:"instance_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if c.instanceService != nil {
		results := make([]services.HealthCheckResult, 0, len(req.InstanceIDs))
		for _, instanceID := range req.InstanceIDs {
			actionResult, err := c.instanceService.HealthCheck(ctx.Request.Context(), instanceID)
			row := services.HealthCheckResult{
				InstanceID: instanceID,
				CheckType:  "agent_mysql",
				CheckTime:  time.Now(),
			}
			if err != nil {
				row.Status = "error"
				row.IsHealthy = false
				row.ErrorMessage = err.Error()
				} else if actionResult != nil {
					row.Status = actionResult.Status
					row.IsHealthy = actionResult.Status == "completed" || actionResult.Status == "healthy" || actionResult.Status == "success"
					if !row.IsHealthy {
						row.ErrorMessage = actionResult.Message
					}
					row.Details.TCPReachable = row.IsHealthy
					row.Details.MySQLAlive = row.IsHealthy
					applyHealthCheckConnectionData(&row, actionResult.Data)
				}
				results = append(results, row)
			}
		utils.SuccessResponse(ctx, results)
		return
	}

	results, err := c.service.BatchHealthCheck(ctx.Request.Context(), req.InstanceIDs)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to execute batch health check", err)
		return
	}

	utils.SuccessResponse(ctx, results)
}

func applyHealthCheckConnectionData(row *services.HealthCheckResult, data interface{}) {
	values, ok := data.(map[string]interface{})
	if !ok || row == nil {
		return
	}
	row.ConnectionHost = stringValue(values["connection_host"])
	row.ConnectionPort = intValue(values["connection_port"])
	row.AgentHost = stringValue(values["agent_host"])
	row.AgentPort = intValue(values["agent_port"])
	row.TargetHost = stringValue(values["target_host"])
	row.TargetPort = intValue(values["target_port"])
	row.TargetUser = stringValue(values["target_user"])
	row.Connection = stringValue(values["connection"])
	row.AgentEndpoint = stringValue(values["agent_endpoint"])
	row.TargetEndpoint = stringValue(values["target_endpoint"])
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func intValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
