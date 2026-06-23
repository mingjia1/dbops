package services

import (
	"reflect"
	"sync"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/repositories"
)

// TestFailureStates_ConcurrentSafe 验证 B9 修复:
// 100 goroutine 并发写同一个 instanceID 的 failureStates,
// 跑 go test -race 必须无 data race 报告.
func TestFailureStates_ConcurrentSafe(t *testing.T) {
	svc := NewHealthCheckService(&repositories.Database{}, "test-key-not-used")

	const goroutines = 100
	const writes = 10

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// 写
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writes; j++ {
				svc.updateFailureState("inst-1", j%2 == 0, "err")
			}
		}()
	}

	// 读
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writes; j++ {
				_ = svc.GetFailureState("inst-1")
			}
		}()
	}

	wg.Wait()

	// 收尾断言: 至少有一条写成功, state 存在
	state := svc.GetFailureState("inst-1")
	if state == nil {
		t.Fatalf("expected state to exist after concurrent writes")
	}
}

func TestBatchHealthCheckTypesOnlyCheckLiveness(t *testing.T) {
	got := batchHealthCheckTypes()
	want := []string{"tcp", "mysql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batch health check types = %v, want %v", got, want)
	}
}
