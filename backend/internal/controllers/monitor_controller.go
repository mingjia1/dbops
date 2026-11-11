package controllers

import (
	"errors"
	"net/http"
	"strings"
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
	// range: 1h(默认) / 24h / 7d, 支持仪表盘按小时/天/周粒度查看.
	var duration time.Duration
	switch ctx.Query("range") {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 1 * time.Hour
	}
	start := end.Add(-duration)

	metrics, err := c.service.QueryMetrics(ctx.Request.Context(), services.MetricQueryRequest{
		InstanceID: instanceID,
		Metrics:    []string{},
		StartTime:  start,
		EndTime:    end,
	})
	if err != nil {
		var credErr *services.InstanceCredentialError
		if errors.As(err, &credErr) {
			// 实例凭据解密失败 (通常是平台加密密钥已更换): 降级为 200 并给出
			// 可操作的提示, 引导用户重新保存实例密码以重新加密。
			utils.SuccessResponse(ctx, gin.H{
				"status":  "credential_error",
				"message": "实例凭据解密失败（平台加密密钥可能已更换）。请在实例管理中重新保存该实例的密码。",
				"metrics": []services.MetricData{},
			})
			return
		}
		var unreachable *services.AgentUnreachableError
		if errors.As(err, &unreachable) {
			// agent 不在线: 降级为 200 + agent_unreachable, 前端展示黄色提示
			// 而非错误 toast。监控面板为自动轮询, 无数据是常态而非故障。
			utils.SuccessResponse(ctx, gin.H{
				"status":  "agent_unreachable",
				"message": "Agent 不在线或网络不可达，无法实时采集指标；请检查主机 Agent 状态。",
				"metrics": []services.MetricData{},
			})
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(err.Error(), "不存在") {
			utils.NotFoundResponse(ctx, "实例不存在: "+instanceID)
			return
		}
		utils.InternalServerErrorResponse(ctx, "查询监控指标失败", err)
		return
	}

	status := c.service.CollectionStatus()
	message := ""
	if status == "not_configured" {
		message = "监控存储未配置，请配置 ClickHouse 或 Agent 指标上报。"
	} else if len(metrics) == 0 {
		if status == "standalone" {
			status = "no_data"
			message = "Agent 未返回实时指标，请检查 Agent 可达性与目标库连通性。"
		} else {
			status = "no_data"
			message = "该实例尚无已采集的监控数据。"
		}
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
		utils.BadRequestResponse(ctx, "监控指标上报请求无效")
		return
	}
	if err := c.service.IngestMetrics(ctx.Request.Context(), req); err != nil {
		utils.ErrorResponse(ctx, http.StatusServiceUnavailable, "写入监控指标失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"status": "ok"})
}
