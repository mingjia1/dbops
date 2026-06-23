package grpc

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Streamer struct{}

func NewStreamer() *Streamer {
	return &Streamer{}
}

type StreamEvent struct {
	TaskID   string `json:"task_id"`
	Progress int    `json:"progress"`
	Stage    string `json:"stage"`
	LogLine  string `json:"log_line"`
	Status   string `json:"status"`
}

type ProgressFunc func(event StreamEvent)

func (s *Streamer) ExecuteAndStream(ctx context.Context, taskID string, command string, args []string, progressFn ProgressFunc) error {
	if progressFn == nil {
		return fmt.Errorf("progress function is required")
	}

	progressFn(StreamEvent{TaskID: taskID, Progress: 0, Stage: "starting", Status: "running"})

	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		progressFn(StreamEvent{TaskID: taskID, Progress: 0, Stage: "error", LogLine: err.Error(), Status: "failed"})
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		progressFn(StreamEvent{TaskID: taskID, Progress: 0, Stage: "error", LogLine: err.Error(), Status: "failed"})
		return fmt.Errorf("start command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		stage := "running"
		progress := 10
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			stage = parseStage(line)
			progress = parseProgress(line, lineCount)
		}

		progressFn(StreamEvent{
			TaskID:   taskID,
			Progress: progress,
			Stage:    stage,
			LogLine:  line,
			Status:   "running",
		})
	}

	if err := cmd.Wait(); err != nil {
		progressFn(StreamEvent{TaskID: taskID, Progress: 100, Stage: "failed", LogLine: err.Error(), Status: "failed"})
		return fmt.Errorf("command failed: %w", err)
	}

	progressFn(StreamEvent{TaskID: taskID, Progress: 100, Stage: "completed", Status: "completed"})
	return nil
}

func parseStage(line string) string {
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return "running"
}

func parseProgress(line string, lineCount int) int {
	parts := strings.SplitN(line, "/", 2)
	if len(parts) == 2 {
		start := strings.LastIndex(parts[0], "[")
		if start >= 0 {
			var current, total int
			fmt.Sscanf(parts[0][start+1:], "%d", &current)
			fmt.Sscanf(parts[1], "%d] ", &total)
			if total > 0 {
				return current * 100 / total
			}
		}
	}
	if lineCount > 0 {
		p := lineCount * 5
		if p > 90 {
			p = 90
		}
		return p
	}
	return 10
}
