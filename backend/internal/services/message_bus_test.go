package services

import (
	"sync"
	"testing"
)

func TestMessageBusConcurrentPublishUnsubscribeNoPanic(t *testing.T) {
	bus := NewMessageBus()
	const taskID = "task-log-stream"
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := bus.Subscribe(taskID)
			defer bus.Unsubscribe(taskID, ch)
			for j := 0; j < 100; j++ {
				select {
				case <-ch:
				default:
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				bus.PublishLog(taskID, "line")
			}
		}()
	}
	wg.Wait()
}
