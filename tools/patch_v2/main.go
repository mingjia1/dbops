package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	path := "../agent/internal/executor/task_executor.go"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read error at %s: %v\n", path, err)
		os.Exit(1)
	}

	content := string(data)

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Fix 1: mysqlExecWithSocketFallback - add logic to return socket output when auth succeeds
	oldComment := "// All failed, return original out with detailed diagnostic error"
	newComment := "// All failed. If socket auth succeeded (non-auth error like ERROR 1064),\n\t// return socket output so callers can detect specific SQL errors.\n\t// Otherwise return TCP output."
	if !strings.Contains(content, oldComment) {
		fmt.Fprintf(os.Stderr, "ERROR: comment not found: %q\n", oldComment)
		os.Exit(1)
	}
	content = strings.ReplaceAll(content, oldComment, newComment)

	// Now add the if-check before the return out statement
	// After the comment change, the code is:
	// // All failed. If socket auth succeeded ...
	// return out, fmt.Errorf(...)
	oldReturn := "\treturn out, fmt.Errorf(\"%s; socket(no-pass): %v - %s; socket(with-pass): %v - %s\","
	insertBefore := "\tif noPassErr != nil && !strings.Contains(strings.ToLower(string(noPassOut)), \"access denied\") {\n\t\treturn noPassOut, fmt.Errorf(\"%s; socket(no-pass): %v - %s; socket(with-pass): %v - %s\","
	if !strings.Contains(content, oldReturn) {
		fmt.Fprintf(os.Stderr, "ERROR: return line not found: %q\n", oldReturn)
		os.Exit(1)
	}
	content = strings.Replace(content, oldReturn, insertBefore, 1)

	// Fix 2: resetReplication - swap order to try REPLICA first
	oldFunc := `func resetReplication(ctx context.Context, config MasterSlaveConfig) ([]byte, error) {
	legacyOut, legacyErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;")
	legacyText := string(legacyOut)
	if legacyErr == nil || strings.Contains(legacyText, "Slave is not configured") || strings.Contains(legacyText, "This server is not configured as slave") {
		return legacyOut, nil
	}
	if !strings.Contains(legacyText, "ERROR 1064") {
		return legacyOut, legacyErr
	}
	replicaOut, replicaErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;")
	replicaText := string(replicaOut)
	if replicaErr == nil || strings.Contains(replicaText, "Replica is not configured") {
		return replicaOut, nil
	}
	return replicaOut, replicaErr
}`

	newFunc := `func resetReplication(ctx context.Context, config MasterSlaveConfig) ([]byte, error) {
	// Try REPLICA syntax first (MySQL 8.4+), fall back to SLAVE for older versions
	replicaOut, replicaErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;")
	replicaText := string(replicaOut)
	if replicaErr == nil || strings.Contains(replicaText, "Replica is not configured") || strings.Contains(replicaText, "This server is not configured as slave") {
		return replicaOut, nil
	}
	if !strings.Contains(replicaText, "ERROR 1064") {
		return replicaOut, replicaErr
	}
	legacyOut, legacyErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;")
	legacyText := string(legacyOut)
	if legacyErr == nil || strings.Contains(legacyText, "Slave is not configured") || strings.Contains(legacyText, "This server is not configured as slave") {
		return legacyOut, nil
	}
	return legacyOut, legacyErr
}`

	if !strings.Contains(content, oldFunc) {
		fmt.Fprintf(os.Stderr, "ERROR: resetReplication function pattern not found\n")
		os.Exit(1)
	}
	content = strings.Replace(content, oldFunc, newFunc, 1)

	// Restore Windows line endings for the file
	content = strings.ReplaceAll(content, "\n", "\r\n")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Write error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done - both fixes applied")
}
