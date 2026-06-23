package services

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type WSHub struct {
	mu      sync.RWMutex
	clients map[string]map[*WSClient]bool
}

type WSClient struct {
	TaskID string
	Send   chan []byte
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[string]map[*WSClient]bool),
	}
}

func (h *WSHub) Register(taskID string, client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[taskID] == nil {
		h.clients[taskID] = make(map[*WSClient]bool)
	}
	h.clients[taskID][client] = true
}

func (h *WSHub) Unregister(taskID string, client *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.clients[taskID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, taskID)
		}
	}
	close(client.Send)
}

func (h *WSHub) Broadcast(taskID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WARN: failed to marshal WS message: %v", err)
		return
	}

	h.mu.RLock()
	clients := h.clients[taskID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			go h.Unregister(taskID, client)
		}
	}
}

func (h *WSHub) ClientCount(taskID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[taskID])
}

func (h *WSHub) TotalClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, clients := range h.clients {
		total += len(clients)
	}
	return total
}

func (h *WSHub) HandleSSE(c *gin.Context) {
	taskID := c.Param("taskID")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskID is required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	client := &WSClient{
		TaskID: taskID,
		Send:   make(chan []byte, 64),
	}
	h.Register(taskID, client)
	defer h.Unregister(taskID, client)

	c.Writer.Flush()

	for {
		select {
		case data, ok := <-client.Send:
			if !ok {
				return
			}
			if _, err := c.Writer.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}
