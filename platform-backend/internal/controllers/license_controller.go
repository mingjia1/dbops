package controllers

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type LicenseController struct {
	service *services.LicenseService
}

func NewLicenseController(service *services.LicenseService) *LicenseController {
	return &LicenseController{service: service}
}

func (c *LicenseController) GetLicenseInfo(ctx *gin.Context) {
	info, err := c.service.LicenseInfo(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to get license info", err)
		return
	}
	utils.SuccessResponse(ctx, info)
}

func (c *LicenseController) UploadLicense(ctx *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		utils.BadRequestResponse(ctx, "Failed to read request body")
		return
	}
	if len(body) == 0 {
		utils.BadRequestResponse(ctx, "License data is required")
		return
	}

	operator, _ := ctx.Get("user_id")
	opStr, _ := operator.(string)

	l, err := c.service.UploadLicense(ctx.Request.Context(), body, opStr)
	if err != nil {
		utils.BadRequestResponse(ctx, err.Error())
		return
	}

	utils.SuccessResponse(ctx, gin.H{
		"message":    "License uploaded successfully",
		"tier":       l.Tier,
		"issued_to":  l.IssuedTo,
		"expires_at": l.ExpiresAt,
	})
}

func (c *LicenseController) GetFeatures(ctx *gin.Context) {
	features := c.service.Features(ctx.Request.Context())
	utils.SuccessResponse(ctx, features)
}
