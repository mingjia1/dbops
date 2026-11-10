package controllers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type ApprovalController struct {
	service *services.ApprovalService
}

func NewApprovalController(service *services.ApprovalService) *ApprovalController {
	return &ApprovalController{service: service}
}

func (c *ApprovalController) ListApprovalRequests(ctx *gin.Context) {
	limitStr := ctx.DefaultQuery("limit", "20")
	offsetStr := ctx.DefaultQuery("offset", "0")
	status := ctx.Query("status")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	var approvalRequests []models.ApprovalRequest
	if status != "" {
		approvalRequests, err = c.service.ListApprovalRequestsByStatus(ctx.Request.Context(), status, limit, offset)
	} else {
		approvalRequests, err = c.service.ListApprovalRequests(ctx.Request.Context(), limit, offset)
	}

	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取审批申请列表失败", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequests)
}

func (c *ApprovalController) GetApprovalRequestByID(ctx *gin.Context) {
	id := ctx.Param("id")

	approvalRequest, err := c.service.GetApprovalRequestByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "审批申请不存在")
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) CreateApprovalRequest(ctx *gin.Context) {
	var req services.CreateApprovalRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	if req.RequesterID == "" {
		req.RequesterID = ctx.GetString("user_id")
	}

	if req.Priority == 0 {
		req.Priority = 1
	}
	if req.ExpiryHours == 0 {
		req.ExpiryHours = 24
	}

	approvalRequest, err := c.service.CreateApprovalRequest(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "创建审批申请失败", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) ApproveRequest(ctx *gin.Context) {
	id := ctx.Param("id")
	approverID := ctx.GetString("user_id")

	var req ApproveRejectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	approvalRequest, err := c.service.ApproveRequest(ctx.Request.Context(), id, approverID, req.Comment)
	if err != nil {
		respondApprovalError(ctx, err, "Failed to approve request")
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) RejectRequest(ctx *gin.Context) {
	id := ctx.Param("id")
	approverID := ctx.GetString("user_id")

	var req ApproveRejectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}

	approvalRequest, err := c.service.RejectRequest(ctx.Request.Context(), id, approverID, req.Comment)
	if err != nil {
		respondApprovalError(ctx, err, "Failed to reject request")
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

type ApproveRejectRequest struct {
	Comment string `json:"comment"`
}

func respondApprovalError(ctx *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, services.ErrApprovalNotPending):
		utils.ErrorResponse(ctx, 400, "审批申请不处于待审批状态", err)
	case strings.Contains(strings.ToLower(err.Error()), "not found"):
		utils.NotFoundResponse(ctx, "审批申请不存在")
	default:
		utils.InternalServerErrorResponse(ctx, fallback, err)
	}
}
