package executor

import (
	"context"
	"fmt"
	"time"
)

func waitForCondition(ctx context.Context, check func() bool, interval, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("condition not met within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func waitForPort(host string, port int, timeout time.Duration) error {
	return waitForCondition(context.Background(),
		func() bool {
			return isPortListening(port)
		},
		500*time.Millisecond,
		timeout,
	)
}

func waitForMySQLReady(host string, port int, user, pass string, timeout time.Duration) error {
	return waitForCondition(context.Background(),
		func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := runMySQLExec(ctx, host, port, user, pass, "SELECT 1")
			return err == nil
		},
		1*time.Second,
		timeout,
	)
}
