package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type RoleController struct {
	service *services.RoleService
}

func NewRoleController(service *services.RoleService) *RoleController {
	return &RoleController{service: service}
}

func (c *RoleController) List(ctx *gin.Context) {
	roles, err := c.service.List(requestContext(ctx))
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list roles", err)
		return
	}
	utils.SuccessResponse(ctx, roles)
}

func (c *RoleController) Create(ctx *gin.Context) {
	var req services.CreateRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	role, err := c.service.Create(requestContext(ctx), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, role)
}

func (c *RoleController) Update(ctx *gin.Context) {
	var req services.UpdateRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	role, err := c.service.Update(requestContext(ctx), ctx.Param("id"), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, role)
}

func (c *RoleController) Delete(ctx *gin.Context) {
	if err := c.service.Delete(requestContext(ctx), ctx.Param("id")); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "Role deleted successfully"})
}
