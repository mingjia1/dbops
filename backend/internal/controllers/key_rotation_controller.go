package controllers

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type KeyRotationController struct {
	service       *services.KeyRotationService
	currentKey    string
}

func NewKeyRotationController(service *services.KeyRotationService, currentKey string) *KeyRotationController {
	return &KeyRotationController{service: service, currentKey: currentKey}
}

func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (c *KeyRotationController) RotateKey(ctx *gin.Context) {
	var req struct {
		NewKey string `json:"new_key" binding:"required"`
		Note   string `json:"note"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "new_key is required")
		return
	}

	operator, _ := ctx.Get("user_id")
	opStr, _ := operator.(string)

	count, err := c.service.RotateKey(ctx.Request.Context(), c.currentKey, req.NewKey, req.Note, opStr)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Key rotation failed", err)
		return
	}

	utils.SuccessResponse(ctx, gin.H{
		"message":              "Key rotation completed",
		"records_re_encrypted": count,
		"new_key_fingerprint":  keyDigest(req.NewKey),
	})
}

func (c *KeyRotationController) ListKeyVersions(ctx *gin.Context) {
	versions, err := c.service.ListKeyVersions(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "Failed to list key versions", err)
		return
	}
	utils.SuccessResponse(ctx, versions)
}
