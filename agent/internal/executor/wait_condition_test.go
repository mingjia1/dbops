package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForCondition_ImmediateTrue(t *testing.T) {
	called := false
	err := waitForCondition(context.Background(), func() bool {
		if !called {
			called = true
			return true
		}
		return true
	}, 10*time.Millisecond, 1*time.Second)
	assert.NoError(t, err)
}

func TestWaitForCondition_Timeout(t *testing.T) {
	err := waitForCondition(context.Background(), func() bool {
		return false
	}, 10*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not met")
}

func TestWaitForCondition_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	err := waitForCondition(ctx, func() bool {
		return false
	}, 10*time.Millisecond, 5*time.Second)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestWaitForCondition_EventuallyTrue(t *testing.T) {
	counter := 0
	err := waitForCondition(context.Background(), func() bool {
		counter++
		return counter >= 3
	}, 10*time.Millisecond, 1*time.Second)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, counter, 3)
}
