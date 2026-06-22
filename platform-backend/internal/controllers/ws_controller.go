package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
)

type WSController struct {
	hub         *services.WSHub
	messageBus  *services.MessageBus
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

	c.hub.HandleSSE(ctx)
}

func (c *WSController) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/tasks/:taskID/stream", c.HandleTaskStream)
}
