package controllers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type UserController struct {
	service *services.UserService
}

func NewUserController(service *services.UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)
	users, err := c.service.List(requestContext(ctx), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取用户列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, users)
}

func (c *UserController) GetByID(ctx *gin.Context) {
	user, err := c.service.GetByID(requestContext(ctx), ctx.Param("id"))
	if err != nil {
		utils.NotFoundResponse(ctx, "用户不存在")
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Create(ctx *gin.Context) {
	var req services.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	user, err := c.service.Create(requestContext(ctx), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Update(ctx *gin.Context) {
	var req services.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	user, err := c.service.Update(requestContext(ctx), ctx.Param("id"), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if ctx.GetString("user_id") == id {
		utils.ErrorResponse(ctx, http.StatusBadRequest, "不能删除自己", nil)
		return
	}
	if err := c.service.Delete(requestContext(ctx), id); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, "删除用户失败", nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "User deleted successfully"})
}

func (c *UserController) Enable(ctx *gin.Context) {
	if err := c.service.Enable(requestContext(ctx), ctx.Param("id")); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "User enabled successfully"})
}

func (c *UserController) Disable(ctx *gin.Context) {
	id := ctx.Param("id")
	if ctx.GetString("user_id") == id {
		utils.ErrorResponse(ctx, http.StatusBadRequest, "不能停用自己", nil)
		return
	}
	if err := c.service.Disable(requestContext(ctx), id); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "User disabled successfully"})
}

func (c *UserController) ResetPassword(ctx *gin.Context) {
	var req services.ResetUserPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	if err := c.service.ResetPassword(requestContext(ctx), ctx.Param("id"), req); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "Password reset successfully"})
}

func (c *UserController) UpdateRoles(ctx *gin.Context) {
	var req services.UpdateUserRolesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "请求参数有误")
		return
	}
	user, err := c.service.UpdateRoles(requestContext(ctx), ctx.Param("id"), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, user)
}

func requestContext(ctx *gin.Context) context.Context {
	base := ctx.Request.Context()
	if userID := ctx.GetString("user_id"); userID != "" {
		base = context.WithValue(base, "user_id", userID)
	}
	return base
}
