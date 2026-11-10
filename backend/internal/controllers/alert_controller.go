package controllers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type AlertController struct {
	service *services.AlertService
}

func NewAlertController(service *services.AlertService) *AlertController {
	return &AlertController{service: service}
}

func (c *AlertController) ListAlertRules(ctx *gin.Context) {
	limit := 20
	offset := 0

	if l := ctx.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	if o := ctx.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	rules, err := c.service.ListAlertRules(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取告警规则列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, rules)
}

func (c *AlertController) CreateAlertRule(ctx *gin.Context) {
	var rule models.AlertRule
	if err := ctx.ShouldBindJSON(&rule); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	if err := c.service.CreateAlertRule(ctx.Request.Context(), &rule); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建告警规则失败", err)
		return
	}

	utils.SuccessResponse(ctx, rule)
}

func (c *AlertController) GetAlertRule(ctx *gin.Context) {
	id := ctx.Param("id")

	rule, err := c.service.GetAlertRuleByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "告警规则不存在")
		return
	}

	utils.SuccessResponse(ctx, rule)
}

func (c *AlertController) UpdateAlertRule(ctx *gin.Context) {
	id := ctx.Param("id")

	var rule models.AlertRule
	if err := ctx.ShouldBindJSON(&rule); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	rule.ID = id
	rule.UpdatedAt = time.Now()

	if err := c.service.UpdateAlertRule(ctx.Request.Context(), &rule); err != nil {
		utils.InternalServerErrorResponse(ctx, "更新告警规则失败", err)
		return
	}

	utils.SuccessResponse(ctx, rule)
}

func (c *AlertController) DeleteAlertRule(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.DeleteAlertRule(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除告警规则失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}

func (c *AlertController) EvaluateAlert(ctx *gin.Context) {
	var req services.EvaluateAlertRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	result, err := c.service.EvaluateAlertRule(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "评估告警规则失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *AlertController) TriggerAlert(ctx *gin.Context) {
	var req services.TriggerAlertRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	result, err := c.service.TriggerAlert(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "触发告警失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *AlertController) GetAlertHistory(ctx *gin.Context) {
	filter := services.AlertHistoryFilter{
		Limit:  20,
		Offset: 0,
	}

	if instanceID := ctx.Query("instance_id"); instanceID != "" {
		filter.InstanceID = instanceID
	}

	if ruleID := ctx.Query("rule_id"); ruleID != "" {
		filter.RuleID = ruleID
	}

	if status := ctx.Query("status"); status != "" {
		filter.Status = status
	}

	if l := ctx.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			filter.Limit = val
		}
	}

	if o := ctx.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			filter.Offset = val
		}
	}

	history, err := c.service.GetAlertHistory(ctx.Request.Context(), filter)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取告警历史失败", err)
		return
	}

	utils.SuccessResponse(ctx, history)
}

func (c *AlertController) SendNotification(ctx *gin.Context) {
	var req services.SendNotificationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	result, err := c.service.SendNotification(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "发送通知失败", err)
		return
	}

	utils.SuccessResponse(ctx, result)
}

func (c *AlertController) ListNotificationChannels(ctx *gin.Context) {
	limit := 20
	offset := 0

	if l := ctx.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	if o := ctx.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	channels, err := c.service.ListNotificationChannels(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取通知渠道列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, channels)
}

func (c *AlertController) CreateNotificationChannel(ctx *gin.Context) {
	var channel models.NotificationChannel
	if err := ctx.ShouldBindJSON(&channel); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()

	if err := c.service.CreateNotificationChannel(ctx.Request.Context(), &channel); err != nil {
		utils.InternalServerErrorResponse(ctx, "创建通知渠道失败", err)
		return
	}

	utils.SuccessResponse(ctx, channel)
}

func (c *AlertController) GetNotificationChannel(ctx *gin.Context) {
	id := ctx.Param("id")

	channel, err := c.service.GetNotificationChannelByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "通知渠道不存在")
		return
	}

	utils.SuccessResponse(ctx, channel)
}

func (c *AlertController) UpdateNotificationChannel(ctx *gin.Context) {
	id := ctx.Param("id")

	var channel models.NotificationChannel
	if err := ctx.ShouldBindJSON(&channel); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}

	channel.ID = id
	channel.UpdatedAt = time.Now()

	if err := c.service.UpdateNotificationChannel(ctx.Request.Context(), &channel); err != nil {
		utils.InternalServerErrorResponse(ctx, "更新通知渠道失败", err)
		return
	}

	utils.SuccessResponse(ctx, channel)
}

func (c *AlertController) DeleteNotificationChannel(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.service.DeleteNotificationChannel(ctx.Request.Context(), id); err != nil {
		utils.InternalServerErrorResponse(ctx, "删除通知渠道失败", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{"id": id, "deleted": true})
}