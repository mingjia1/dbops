package controllers

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type AuthController struct {
	authService *services.AuthService
}

func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

func (c *AuthController) Login(ctx *gin.Context) {
	var req services.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	resp, err := c.authService.Login(ctx.Request.Context(), req)
	if err != nil {
		utils.UnauthorizedResponse(ctx, "Authentication failed")
		return
	}

	utils.SuccessResponse(ctx, resp)
}

func (c *AuthController) Register(ctx *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email" binding:"required,email"`
		Role     string `json:"role" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}

	if err := c.authService.Register(ctx.Request.Context(), req.Username, req.Password, req.Email, req.Role); err != nil {
		utils.ErrorResponse(ctx, 400, err.Error(), nil)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "User registered successfully",
	})
}

func (c *AuthController) ValidateToken(ctx *gin.Context) {
	token := ctx.GetHeader("Authorization")
	if token == "" {
		utils.UnauthorizedResponse(ctx, "Missing authorization token")
		ctx.Abort()
		return
	}

	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	claims, err := c.authService.ValidateToken(token)
	if err != nil {
		utils.UnauthorizedResponse(ctx, "Invalid token")
		ctx.Abort()
		return
	}

	ctx.Set("user_id", claims.UserID)
	ctx.Set("username", claims.Username)
	ctx.Set("role", claims.Role)
	ctx.Next()
}