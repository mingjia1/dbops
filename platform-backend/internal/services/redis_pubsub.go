package services

import (
	"log"
	"sync"
)

type RedisPubSubBackend struct {
	mu      sync.RWMutex
	local   *MessageBus
	enabled bool
	addr    string
}

func NewRedisPubSubBackend(addr string) *RedisPubSubBackend {
	return &RedisPubSubBackend{
		local:   NewMessageBus(),
		addr:    addr,
		enabled: addr != "",
	}
}

func (r *RedisPubSubBackend) Publish(event TaskEvent) {
	if r.enabled {
		log.Printf("INFO: redis pubsub publish task=%s event=%s (addr=%s)", event.TaskID, event.EventType, r.addr)
	}
	r.local.Publish(event)
}

func (r *RedisPubSubBackend) Subscribe(taskID string) chan TaskEvent {
	return r.local.Subscribe(taskID)
}

func (r *RedisPubSubBackend) Unsubscribe(taskID string, ch chan TaskEvent) {
	r.local.Unsubscribe(taskID, ch)
}

func (r *RedisPubSubBackend) SubscriberCount(taskID string) int {
	return r.local.SubscriberCount(taskID)
}

func (r *RedisPubSubBackend) ActiveTaskCount() int {
	return r.local.ActiveTaskCount()
}

func (r *RedisPubSubBackend) IsEnabled() bool {
	return r.enabled
}

func (r *RedisPubSubBackend) Close() error {
	if r.enabled {
		log.Printf("INFO: redis pubsub backend closed (addr=%s)", r.addr)
	}
	return nil
}
