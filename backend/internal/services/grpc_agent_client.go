package services

import (
	"context"
	"fmt"
	"log"
	"time"
)

type GRPCAgentClient struct {
	agentAddr string
	timeout   time.Duration
}

func NewGRPCAgentClient(agentAddr string) *GRPCAgentClient {
	return &GRPCAgentClient{
		agentAddr: agentAddr,
		timeout:   30 * time.Minute,
	}
}

type GRPCTaskResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message"`
}

func (c *GRPCAgentClient) ExecuteTaskStream(ctx context.Context, taskID string, config map[string]interface{}, bus *MessageBus) (*GRPCTaskResult, error) {
	if c.agentAddr == "" {
		return nil, fmt.Errorf("agent address is empty")
	}

	log.Printf("INFO: gRPC ExecuteTaskStream task=%s agent=%s", taskID, c.agentAddr)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	progressCh := bus.Subscribe(taskID)
	defer bus.Unsubscribe(taskID, progressCh)

	result := &GRPCTaskResult{
		TaskID: taskID,
		Status: "running",
	}

	for {
		select {
		case event := <-progressCh:
			result.Progress = event.Progress
			result.Status = event.Status
			if event.Status == "completed" || event.Status == "failed" {
				return result, nil
			}
		case <-ctx.Done():
			result.Status = "timeout"
			return result, fmt.Errorf("task %s timed out", taskID)
		}
	}
}
