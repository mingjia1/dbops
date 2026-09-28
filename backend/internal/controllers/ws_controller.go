package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
)

type WSController struct {
	hub        *services.WSHub
	messageBus *services.MessageBus
}

func NewWSController(hub *services.WSHub, bus *services.MessageBus) *WSController {
	return &WSController{
		hub:        hub,
		messageBus: bus,
	}
}

func (c *WSController) HandleTaskStream(ctx *gin.Context) {
	taskID := ctx.Param("taskID")
	if taskID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "taskID is required"})
		return
	}

	ch := c.messageBus.Subscribe(taskID)
	defer c.messageBus.Unsubscribe(taskID, ch)

	done := ctx.Request.Context().Done()
	go func() {
		for {
			select {
			case <-done:
				return
			case event := <-ch:
				msg := services.WSMessage{
					Type: event.EventType,
					Data: event,
				}
				c.hub.Broadcast(taskID, msg)
			}
		}
	}()

	c.hub.HandleSSE(ctx)
}

func (c *WSController) RegisterRoutes(r *gin.RouterGroup, authMiddleware ...gin.HandlerFunc) {
	handler := gin.HandlerFunc(c.HandleTaskStream)
	// P0-6: SSE 走专用鉴权变体 (支持一次性 ticket), 不吃组上的默认 ValidateToken.
	mw := authMiddleware[0]
	r.GET("/tasks/stream/:taskID", mw, handler)
}
