package controllers

import (
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
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
	body, err := licensePayload(ctx)
	if err != nil {
		utils.BadRequestResponse(ctx, err.Error())
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

func licensePayload(ctx *gin.Context) ([]byte, error) {
	if strings.HasPrefix(ctx.GetHeader("Content-Type"), "multipart/form-data") {
		file, err := ctx.FormFile("license")
		if err != nil {
			return nil, err
		}
		opened, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer opened.Close()
		return io.ReadAll(io.LimitReader(opened, 1<<20))
	}

	return io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
}

func (c *LicenseController) GetFeatures(ctx *gin.Context) {
	features := c.service.Features(ctx.Request.Context())
	utils.SuccessResponse(ctx, features)
}
