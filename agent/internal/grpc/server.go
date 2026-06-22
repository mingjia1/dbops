package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, taskID string, config map[string]interface{}, progressFn func(progress int, stage, logLine, status string)) error
}

type AgentGRPCServer struct {
	executor    TaskExecutor
	mu          sync.RWMutex
	activeTasks map[string]context.CancelFunc
}

func NewAgentGRPCServer(executor TaskExecutor) *AgentGRPCServer {
	return &AgentGRPCServer{
		executor:    executor,
		activeTasks: make(map[string]context.CancelFunc),
	}
}

func (s *AgentGRPCServer) ExecuteTaskStream(taskID string, config map[string]interface{}, progressFn func(progress int, stage, logLine, status string)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.mu.Lock()
	s.activeTasks[taskID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeTasks, taskID)
		s.mu.Unlock()
	}()

	return s.executor.ExecuteTask(ctx, taskID, config, progressFn)
}

func (s *AgentGRPCServer) HealthCheck() (string, string) {
	return "healthy", "1.0.0"
}

func (s *AgentGRPCServer) ActiveTaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeTasks)
}

func (s *AgentGRPCServer) CancelTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.activeTasks[taskID]; ok {
		cancel()
		return true
	}
	return false
}

func (s *AgentGRPCServer) Start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	log.Printf("Agent gRPC server listening on %s", addr)
	return serveGRPC(lis, s)
}

func serveGRPC(lis net.Listener, server *AgentGRPCServer) error {
	return fmt.Errorf("gRPC transport not yet configured on %s", lis.Addr().String())
}
