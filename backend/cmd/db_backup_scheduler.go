package main

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// dbBackupScheduler P0-B: 平台元数据库定时快照调度器.
//
// 背景: 元数据库存着全部被管实例的加密凭据 / 主机信息 / 审计链, 此前只有
// 手动 `main.go db-backup` 子命令, 平台自身裸奔. 这里在进程内按
// db_backup_interval 周期复用既有的 backupSQLite / backupMySQL 做快照与轮转.
//
// 设计要点:
//   - interval<=0 表示关闭 (仍是显式可关的开关, 不靠隐式状态推导);
//   - 备份失败只记 Error + 计数, 不阻断服务; 计数通过 /metrics 暴露;
//   - Stop() 幂等且等待进行中的备份收尾, 供优雅停机调用.
type dbBackupScheduler struct {
	interval time.Duration
	keep     int
	outDir   string
	backup   func() error
	log      *zap.Logger

	mu        sync.Mutex
	successes int64
	failures  int64
	lastOK    time.Time

	stopCh  chan struct{}
	doneCh  chan struct{}
	once    sync.Once
	started bool
}

// newDBBackupScheduler interval<=0 时返回 nil (调用方据此跳过启动).
func newDBBackupScheduler(interval time.Duration, keep int, outDir string, backup func() error, log *zap.Logger) *dbBackupScheduler {
	if interval <= 0 {
		return nil
	}
	if keep < 1 {
		keep = 7
	}
	return &dbBackupScheduler{
		interval: interval,
		keep:     keep,
		outDir:   outDir,
		backup:   backup,
		log:      log,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (s *dbBackupScheduler) Start() {
	if s == nil || s.started {
		return
	}
	s.started = true
	go s.loop()
	s.log.Info(fmt.Sprintf("Metadata DB snapshot scheduler started: interval=%s keep=%d dir=%s",
		s.interval, s.keep, s.outDir))
}

// Stop 等待当前快照收尾后退出; 重复调用安全.
func (s *dbBackupScheduler) Stop() {
	if s == nil || !s.started {
		return
	}
	s.once.Do(func() { close(s.stopCh) })
	<-s.doneCh
}

func (s *dbBackupScheduler) loop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.RunOnce()
		case <-s.stopCh:
			return
		}
	}
}

// RunOnce 执行一次快照并记录结果 (导出以便单测直接驱动).
func (s *dbBackupScheduler) RunOnce() {
	start := time.Now()
	err := s.backup()
	s.mu.Lock()
	if err != nil {
		s.failures++
	} else {
		s.successes++
		s.lastOK = time.Now()
	}
	s.mu.Unlock()
	if err != nil {
		s.log.Error(fmt.Sprintf("Metadata DB snapshot failed (dir=%s): %s", s.outDir, err.Error()),
			zap.Duration("took", time.Since(start)))
		return
	}
	s.log.Info(fmt.Sprintf("Metadata DB snapshot OK (dir=%s, keep=%d)", s.outDir, s.keep),
		zap.Duration("took", time.Since(start)))
}

// Stats 返回 (成功次数, 失败次数, 最近成功 unix 秒).
func (s *dbBackupScheduler) Stats() (int64, int64, int64) {
	if s == nil {
		return 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var lastOK int64
	if !s.lastOK.IsZero() {
		lastOK = s.lastOK.Unix()
	}
	return s.successes, s.failures, lastOK
}
