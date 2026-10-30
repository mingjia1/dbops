package services

import (
	"log"
	"sync"
	"time"
)

// P1-D: 后台任务协调器.
//
// 此前部署编排 / 主机扫描 / 连接测试 / 升级派发都是 `go func()` 裸跑 ——
// 进程收到 SIGTERM 时 HTTP 连接会被 srv.Shutdown 排空, 但这些任务可能还在
// 写 taskRepo / 更新状态, 直接退出会留下"任务卡在 running"的脏状态.
//
// 这里提供一个进程级登记点: 需要"停机前收尾"的任务用 GoBackground 启动,
// 停机时主流程调用 WaitForBackground 做**有界**等待 (超时就放行并告警,
// 绝不因为单个任务卡死而无限期挂住停机).
var (
	bgMu sync.Mutex
	bgWg sync.WaitGroup
)

// GoBackground 启动一个纳入停机等待的后台任务.
func GoBackground(fn func()) {
	bgMu.Lock()
	bgWg.Add(1)
	bgMu.Unlock()
	go func() {
		defer func() {
			bgWg.Done()
			// 任务 panic 不能拖垮进程 (后台任务无调用方能接收错误).
			if r := recover(); r != nil {
				logPanic(r)
			}
		}()
		fn()
	}()
}

// WaitForBackground 有界等待所有已登记后台任务结束.
// 返回 true=全部收尾完成; false=超时仍有任务在跑 (调用方应告警后继续停机).
func WaitForBackground(timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	done := make(chan struct{})
	go func() {
		bgWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// ResetBackgroundForTest 仅测试用: 丢弃可能残留的等待 goroutine 视图.
func ResetBackgroundForTest() {
	bgMu.Lock()
	defer bgMu.Unlock()
	bgWg = sync.WaitGroup{}
}

func logPanic(r interface{}) {
	log.Printf("WARN: background task panicked: %v", r)
}
