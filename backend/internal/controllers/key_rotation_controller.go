package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type KeyRotationController struct {
	service *services.KeyRotationService

	// keyMu/key: 轮转成功后运行时切换当前密钥, 保证进程内后续解密立即使用新密钥;
	// 持久化仍需运维将新密钥写入 DBOPS_ENCRYPTION_KEY 并重启 (响应中已提示)。
	keyMu sync.Mutex
	key   string
}

func NewKeyRotationController(service *services.KeyRotationService, currentKey string) *KeyRotationController {
	return &KeyRotationController{service: service, key: currentKey}
}

func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (c *KeyRotationController) currentKey() string {
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	return c.key
}

func (c *KeyRotationController) setKey(newKey string) {
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	c.key = newKey
}

func (c *KeyRotationController) RotateKey(ctx *gin.Context) {
	var req struct {
		NewKey string `json:"new_key" binding:"required"`
		Note   string `json:"note"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(ctx, "缺少 new_key")
		return
	}

	operator, _ := ctx.Get("user_id")
	opStr, _ := operator.(string)

	oldKey := c.currentKey()
	count, err := c.service.RotateKey(ctx.Request.Context(), oldKey, req.NewKey, req.Note, opStr)
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "密钥轮换失败", err)
		return
	}

	// 运行时立即切换, 进程内后续加解密使用新密钥; 持久化由运维更新环境变量
	c.setKey(req.NewKey)
	log.Printf("INFO: encryption key rotated by %s, %d records re-encrypted", opStr, count)

	utils.SuccessResponse(ctx, gin.H{
		"message":               "Key rotation completed. Update DBOPS_ENCRYPTION_KEY to the new key and restart the backend to persist it.",
		"records_re_encrypted":  count,
		"new_key_fingerprint":   keyDigest(req.NewKey),
	})
}

func (c *KeyRotationController) ListKeyVersions(ctx *gin.Context) {
	versions, err := c.service.ListKeyVersions(ctx.Request.Context())
	if err != nil {
		utils.InternalServerErrorResponse(ctx, "获取密钥版本列表失败", err)
		return
	}
	utils.SuccessResponse(ctx, versions)
}
