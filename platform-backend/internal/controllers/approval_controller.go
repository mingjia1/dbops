package controllers

import (
	"strconv"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
		utils.InternalServerErrorResponse(ctx, "Failed to list approval requests", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequests)
}

func (c *ApprovalController) GetApprovalRequestByID(ctx *gin.Context) {
	id := ctx.Param("id")

	approvalRequest, err := c.service.GetApprovalRequestByID(ctx.Request.Context(), id)
	if err != nil {
		utils.NotFoundResponse(ctx, "Approval request not found")
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) CreateApprovalRequest(ctx *gin.Context) {
	var req services.CreateApprovalRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if req.Priority == 0 {
		req.Priority = 1
	}
	if req.ExpiryHours == 0 {
		req.ExpiryHours = 24
	}

	approvalRequest, err := c.service.CreateApprovalRequest(ctx.Request.Context(), req)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to create approval request", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) ApproveRequest(ctx *gin.Context) {
	id := ctx.Param("id")
	approverID := ctx.GetString("user_id")

	var req ApproveRejectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	approvalRequest, err := c.service.ApproveRequest(ctx.Request.Context(), id, approverID, req.Comment)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to approve request", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

func (c *ApprovalController) RejectRequest(ctx *gin.Context) {
	id := ctx.Param("id")
	approverID := ctx.GetString("user_id")

	var req ApproveRejectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	approvalRequest, err := c.service.RejectRequest(ctx.Request.Context(), id, approverID, req.Comment)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to reject request", err)
		return
	}

	utils.SuccessResponse(ctx, approvalRequest)
}

type ApproveRejectRequest struct {
	Comment string `json:"comment"`
}