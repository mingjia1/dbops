package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// P0-B: interval<=0 必须返回 nil (关闭调度), 不能悄悄起协程.
func TestNewDBBackupScheduler_Disabled(t *testing.T) {
	s := newDBBackupScheduler(0, 7, "/tmp/out", func() error { return nil }, zap.NewNop())
	if s != nil {
		t.Fatalf("scheduler = %v, want nil when interval is 0", s)
	}
	s.Stop() // nil 安全
}

func TestDBBackupScheduler_RunOnceRecordsStats(t *testing.T) {
	var calls atomic.Int64
	fail := false
	s := newDBBackupScheduler(time.Hour, 3, "/tmp/out", func() error {
		calls.Add(1)
		if fail {
			return errors.New("boom")
		}
		return nil
	}, zap.NewNop())
	if s == nil {
		t.Fatal("scheduler is nil, want enabled for 1h interval")
	}

	s.RunOnce()
	s.RunOnce()
	if ok, failN, last := s.Stats(); ok != 2 || failN != 0 || last == 0 {
		t.Errorf("Stats() = (%d,%d,%d), want (2,0,nonzero)", ok, failN, last)
	}

	fail = true
	s.RunOnce()
	if ok, failN, last := s.Stats(); ok != 2 || failN != 1 {
		t.Errorf("Stats() = (%d,%d,%d), want (2,1,*)", ok, failN, last)
	}
	if calls.Load() != 3 {
		t.Errorf("backup called %d times, want 3", calls.Load())
	}
}

// 启动后 Stop 必须能等到协程退出 (优雅停机依赖).
func TestDBBackupScheduler_StartStop(t *testing.T) {
	s := newDBBackupScheduler(10*time.Millisecond, 7, "/tmp/out", func() error { return nil }, zap.NewNop())
	if s == nil {
		t.Fatal("scheduler is nil")
	}
	s.Start()
	s.Start() // 幂等
	time.Sleep(30 * time.Millisecond)
	s.Stop()
	s.Stop() // 幂等
	if ok, _, _ := s.Stats(); ok < 1 {
		t.Errorf("successes = %d, want >= 1 after ticks", ok)
	}
}
