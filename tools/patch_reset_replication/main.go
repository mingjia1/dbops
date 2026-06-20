package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	path := "agent/internal/executor/task_executor.go"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
		os.Exit(1)
	}

	content := string(data)

	old1 := `mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;").CombinedOutput()`
	new1 := `mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;")`

	old2 := `mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;").CombinedOutput()`
	new2 := `mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;")`

	count1 := strings.Count(content, old1)
	count2 := strings.Count(content, old2)

	if count1 != 1 {
		fmt.Fprintf(os.Stderr, "ERROR: found %d occurrences of pattern 1 (expected 1)\n", count1)
		os.Exit(1)
	}
	if count2 != 1 {
		fmt.Fprintf(os.Stderr, "ERROR: found %d occurrences of pattern 2 (expected 1)\n", count2)
		os.Exit(1)
	}

	content = strings.ReplaceAll(content, old1, new1)
	content = strings.ReplaceAll(content, old2, new2)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Write error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done - resetReplication patched successfully")
}
