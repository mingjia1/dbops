package services

import (
	"sync/atomic"
	"testing"
	"time"
)

// P1-D: 登记的后台任务必须被 WaitForBackground 等到.
func TestWaitForBackground_WaitsForTasks(t *testing.T) {
	ResetBackgroundForTest()
	var done atomic.Bool
	GoBackground(func() {
		time.Sleep(50 * time.Millisecond)
		done.Store(true)
	})
	if !WaitForBackground(2 * time.Second) {
		t.Fatal("WaitForBackground = false, want true (task should finish in time)")
	}
	if !done.Load() {
		t.Error("task did not finish before Wait returned")
	}
}

// 超时必须返回 false 而不是永远阻塞 (停机不能被单个卡死任务挂住).
func TestWaitForBackground_TimesOut(t *testing.T) {
	ResetBackgroundForTest()
	release := make(chan struct{})
	defer close(release)
	GoBackground(func() { <-release })

	if WaitForBackground(50 * time.Millisecond) {
		t.Fatal("WaitForBackground = true, want false while a task is stuck")
	}
}

// 后台任务 panic 不能让进程崩溃, 也不能泄漏计数.
func TestGoBackground_RecoversPanic(t *testing.T) {
	ResetBackgroundForTest()
	GoBackground(func() { panic("boom") })
	if !WaitForBackground(2 * time.Second) {
		t.Fatal("panicked task should still be marked done")
	}
}
