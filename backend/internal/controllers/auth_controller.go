package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
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
	req.IPAddress = ctx.ClientIP()
	req.UserAgent = ctx.Request.UserAgent()

	resp, err := c.authService.Login(ctx.Request.Context(), req)
	if err != nil {
		utils.UnauthorizedResponse(ctx, "Authentication failed")
		return
	}

	// 同时写 Set-Cookie (HttpOnly, SameSite=Lax), 让浏览器自动带,
	// 前端 axios 配 withCredentials=true 后能跨 fetch 复用同一会话.
	// 旧前端走 localStorage 也兼容 (LoginResponse 仍返 token).
	if resp != nil && resp.Token != "" {
		// 7 天, 与现有 auth_service token expiry 对齐.
		ctx.SetSameSite(http.SameSiteLaxMode)
		ctx.SetCookie("auth_token", resp.Token, 7*24*3600, "/", "", shouldUseSecureCookie(ctx), true)
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
	req.Role = "operator"

	if err := c.authService.Register(ctx.Request.Context(), req.Username, req.Password, req.Email, req.Role); err != nil {
		utils.ErrorResponse(ctx, 400, err.Error(), nil)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "User registered successfully",
	})
}

// Logout 清 HttpOnly cookie. 客户端收到 200 后清 localStorage user 信息即可.
func (c *AuthController) Logout(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("auth_token", "", -1, "/", "", shouldUseSecureCookie(ctx), true)
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success"})
}

func (c *AuthController) Me(ctx *gin.Context) {
	user, err := c.authService.CurrentUser(requestContext(ctx), ctx.GetString("user_id"))
	if err != nil {
		utils.UnauthorizedResponse(ctx, "Invalid user session")
		return
	}
	utils.SuccessResponse(ctx, user)
}

func (c *AuthController) ChangePassword(ctx *gin.Context) {
	var req services.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	userID := ctx.GetString("user_id")
	if err := c.authService.ChangePassword(requestContext(ctx), userID, req); err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"message": "Password changed successfully"})
}

func (c *AuthController) ResetAllPasswords(ctx *gin.Context) {
	var req services.ResetAllPasswordsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "Invalid request parameters")
		return
	}
	updated, err := c.authService.ResetAllPasswords(requestContext(ctx), req)
	if err != nil {
		utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResponse(ctx, gin.H{"updated": updated})
}

func (c *AuthController) ValidateToken(ctx *gin.Context) {
	// B1 (cookie): 优先读 Authorization header, fallback 到 HttpOnly cookie "auth_token".
	// 这样无 cookie 老前端 / curl 还能用 Bearer, 而启用 cookie 的浏览器请求无需暴露 token.
	token := ctx.GetHeader("Authorization")
	if token == "" {
		if c, err := ctx.Cookie("auth_token"); err == nil && c != "" {
			token = "Bearer " + c
		}
	}
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
	ctx.Set("permissions", claims.Permissions)
	ctx.Next()
}

func shouldUseSecureCookie(ctx *gin.Context) bool {
	if ctx.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https")
}
