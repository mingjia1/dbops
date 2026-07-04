package services

import (
	"fmt"
	"log"
	"sync"
)

type TaskEvent struct {
	TaskID    string            `json:"task_id"`
	EventType string            `json:"event_type"`
	Progress  int               `json:"progress"`
	Stage     string            `json:"stage"`
	LogLine   string            `json:"log_line"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type MessageBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan TaskEvent
}

func NewMessageBus() *MessageBus {
	return &MessageBus{
		subscribers: make(map[string][]chan TaskEvent),
	}
}

func (b *MessageBus) Subscribe(taskID string) chan TaskEvent {
	ch := make(chan TaskEvent, 64)
	b.mu.Lock()
	b.subscribers[taskID] = append(b.subscribers[taskID], ch)
	b.mu.Unlock()
	return ch
}

func (b *MessageBus) Unsubscribe(taskID string, ch chan TaskEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
	if len(b.subscribers[taskID]) == 0 {
		delete(b.subscribers, taskID)
	}
}

func (b *MessageBus) Publish(event TaskEvent) {
	b.mu.RLock()
	subs := b.subscribers[event.TaskID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			log.Printf("WARN: message bus subscriber buffer full for task %s, dropping event", event.TaskID)
		}
	}
}

func (b *MessageBus) SubscriberCount(taskID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[taskID])
}

func (b *MessageBus) ActiveTaskCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

func (b *MessageBus) PublishProgress(taskID string, progress int, stage, status string) {
	b.Publish(TaskEvent{
		TaskID:    taskID,
		EventType: "progress",
		Progress:  progress,
		Stage:     stage,
		Status:    status,
	})
}

func (b *MessageBus) PublishLog(taskID string, logLine string) {
	b.Publish(TaskEvent{
		TaskID:    taskID,
		EventType: "log",
		LogLine:   logLine,
	})
}

func (b *MessageBus) PublishStatus(taskID string, status string) {
	b.Publish(TaskEvent{
		TaskID:    taskID,
		EventType: "status",
		Status:    status,
	})
}

func (b *MessageBus) PublishStepEvent(taskID, stepName, stepStatus, stepMessage string) {
	b.Publish(TaskEvent{
		TaskID:    taskID,
		EventType: "step",
		Stage:     stepName,
		Status:    stepStatus,
		Metadata: map[string]string{
			"step_name":    stepName,
			"step_status":  stepStatus,
			"step_message": stepMessage,
		},
	})
}

func (b *MessageBus) WaitForCompletion(taskID string, ch chan TaskEvent) (TaskEvent, error) {
	for event := range ch {
		if event.Status == "completed" || event.Status == "failed" {
			return event, nil
		}
	}
	return TaskEvent{}, fmt.Errorf("task %s channel closed without completion", taskID)
}
