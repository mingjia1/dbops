package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type UserController struct {
	service *services.UserService
}

func NewUserController(service *services.UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) List(ctx *gin.Context) {
	limit, offset := parsePagination(ctx)
	users, err := c.service.List(ctx.Request.Context(), limit, offset)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list users", err)
		return
	}
	utils.SuccessResponse(ctx, users)
}

func (c *UserController) GetByID(ctx *gin.Context) {
	user, err := c.service.GetByID(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		utils.NotFoundResponse(ctx, "User not found")
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Create(ctx *gin.Context) {
	var req services.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	user, err := c.service.Create(ctx.Request.Context(), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Update(ctx *gin.Context) {
	var req services.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	user, err := c.service.Update(ctx.Request.Context(), ctx.Param("id"), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *UserController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if ctx.GetString("user_id") == id {
		utils.ErrorResponse(ctx, http.StatusBadRequest, "cannot delete yourself", nil)
		return
	}
	if err := c.service.Delete(ctx.Request.Context(), id); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "User deleted successfully"})
}
