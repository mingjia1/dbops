package services

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageBus_PublishSubscribe(t *testing.T) {
	bus := NewMessageBus()
	ch := bus.Subscribe("task-001")

	go func() {
		bus.PublishProgress("task-001", 50, "installing", "running")
	}()

	select {
	case event := <-ch:
		assert.Equal(t, "task-001", event.TaskID)
		assert.Equal(t, "progress", event.EventType)
		assert.Equal(t, 50, event.Progress)
		assert.Equal(t, "installing", event.Stage)
		assert.Equal(t, "running", event.Status)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestMessageBus_MultipleSubscribers(t *testing.T) {
	bus := NewMessageBus()
	ch1 := bus.Subscribe("task-001")
	ch2 := bus.Subscribe("task-001")

	assert.Equal(t, 2, bus.SubscriberCount("task-001"))

	bus.PublishLog("task-001", "hello")

	ev1 := <-ch1
	ev2 := <-ch2
	assert.Equal(t, "hello", ev1.LogLine)
	assert.Equal(t, "hello", ev2.LogLine)
}

func TestMessageBus_Unsubscribe(t *testing.T) {
	bus := NewMessageBus()
	ch := bus.Subscribe("task-001")
	assert.Equal(t, 1, bus.SubscriberCount("task-001"))

	bus.Unsubscribe("task-001", ch)
	assert.Equal(t, 0, bus.SubscriberCount("task-001"))
}

func TestMessageBus_PublishNoSubscribers(t *testing.T) {
	bus := NewMessageBus()
	bus.PublishProgress("task-nonexistent", 100, "done", "completed")
}

func TestMessageBus_WaitForCompletion(t *testing.T) {
	bus := NewMessageBus()
	ch := bus.Subscribe("task-002")

	var wg sync.WaitGroup
	wg.Add(1)
	var result TaskEvent
	var resultErr error

	go func() {
		defer wg.Done()
		result, resultErr = bus.WaitForCompletion("task-002", ch)
	}()

	bus.PublishProgress("task-002", 50, "running", "running")
	bus.PublishProgress("task-002", 100, "done", "completed")

	wg.Wait()
	require.NoError(t, resultErr)
	assert.Equal(t, "completed", result.Status)
}

func TestMessageBus_ActiveTaskCount(t *testing.T) {
	bus := NewMessageBus()
	_ = bus.Subscribe("task-a")
	_ = bus.Subscribe("task-b")
	_ = bus.Subscribe("task-a")

	assert.Equal(t, 2, bus.ActiveTaskCount())
}

func TestWSHub_RegisterUnregister(t *testing.T) {
	hub := NewWSHub()
	client := &WSClient{TaskID: "t1", Send: make(chan []byte, 16)}

	hub.Register("t1", client)
	assert.Equal(t, 1, hub.ClientCount("t1"))
	assert.Equal(t, 1, hub.TotalClientCount())

	hub.Unregister("t1", client)
	assert.Equal(t, 0, hub.ClientCount("t1"))
}

func TestWSHub_Broadcast(t *testing.T) {
	hub := NewWSHub()
	client := &WSClient{TaskID: "t1", Send: make(chan []byte, 16)}

	hub.Register("t1", client)

	hub.Broadcast("t1", WSMessage{
		Type: "progress",
		Data: map[string]interface{}{"progress": 75},
	})

	select {
	case data := <-client.Send:
		assert.Contains(t, string(data), "progress")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestWSHub_BroadcastToWrongTask(t *testing.T) {
	hub := NewWSHub()
	client := &WSClient{TaskID: "t1", Send: make(chan []byte, 16)}
	hub.Register("t1", client)

	hub.Broadcast("t2", WSMessage{Type: "progress"})

	select {
	case <-client.Send:
		t.Fatal("should not receive message for different task")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWSHub_TotalClientCount(t *testing.T) {
	hub := NewWSHub()
	hub.Register("t1", &WSClient{TaskID: "t1", Send: make(chan []byte, 16)})
	hub.Register("t1", &WSClient{TaskID: "t1", Send: make(chan []byte, 16)})
	hub.Register("t2", &WSClient{TaskID: "t2", Send: make(chan []byte, 16)})

	assert.Equal(t, 3, hub.TotalClientCount())
}
