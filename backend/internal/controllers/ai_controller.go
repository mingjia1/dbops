package controllers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/aiprovider"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type AIController struct {
	service *services.AIService
}

func NewAIController(service *services.AIService) *AIController {
	return &AIController{service: service}
}

type DiagnosisRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}

type ChatMessageRequest struct {
	Role    string `json:"role" binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"required"`
}

type ChatRequest struct {
	Messages []ChatMessageRequest `json:"messages" binding:"required,min=1,dive"`
}

// Chat forwards an operations chat conversation to the configured AI provider.
func (c *AIController) Chat(ctx *gin.Context) {
	var req ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	messages := make([]aiprovider.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, aiprovider.Message{Role: m.Role, Content: m.Content})
	}
	resp, err := c.service.Chat(ctx.Request.Context(), messages)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "AI 对话失败", err)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"reply": resp.Content, "model": resp.Model})
}

func (c *AIController) Diagnosis(ctx *gin.Context) {
	var req DiagnosisRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	result, err := c.service.Diagnosis(ctx.Request.Context(), req.InstanceID)
	if err != nil {
		// provider 未配置属环境/配置问题, 返回 400 并给出配置指引而非笼统 500
		if strings.Contains(err.Error(), "not configured") {
			utils.BadRequestResponse(ctx, "AI 功能未启用: 请设置 DBOPS_AI_BASE_URL 与 DBOPS_AI_API_KEY")
			return
		}
		utils.InternalServerErrorResponse(ctx, "AI 诊断失败", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *AIController) ListDiagnoses(ctx *gin.Context) {
	instanceID := ctx.Query("instance_id")
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
	list, err := c.service.ListDiagnoses(ctx.Request.Context(), instanceID, limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取诊断记录列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *AIController) GetDiagnosis(ctx *gin.Context) {
	id := ctx.Param("id")
	d, err := c.service.GetDiagnosis(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "诊断记录不存在")
		return
	}
	utils.SuccessResponse(ctx, d)
}

// ListDiagnoses and GetDiagnosis — add to AIService

type SQLAdviceRequest struct {
	SQLText      string `json:"sql_text" binding:"required"`
	ExplainPlan  string `json:"explain"`
	TableSchema  string `json:"schema"`
}

func (c *AIController) SQLAdvice(ctx *gin.Context) {
	var req SQLAdviceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求体解析失败: "+err.Error())
		return
	}
	result, err := c.service.SQLAdvice(ctx.Request.Context(), req.SQLText, req.ExplainPlan, req.TableSchema)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "AI SQL 优化失败", err)
		return
	}
	utils.SuccessResponse(ctx, result)
}

func (c *AIController) ListSQLAdvice(ctx *gin.Context) {
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
	list, err := c.service.ListSQLAdvice(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取 SQL 优化建议列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, list)
}

func (c *AIController) GetSQLAdvice(ctx *gin.Context) {
	id := ctx.Param("id")
	a, err := c.service.GetSQLAdvice(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "SQL 优化建议不存在")
		return
	}
	utils.SuccessResponse(ctx, a)
}
