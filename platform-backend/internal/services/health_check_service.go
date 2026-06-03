package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type HealthCheckService struct {
	db              *repositories.Database
	instanceRepo    *repositories.InstanceRepository
	checkInterval   time.Duration
	timeout         time.Duration
	failureThreshold int
}

func NewHealthCheckService(db *repositories.Database) *HealthCheckService {
	return &HealthCheckService{
		db:               db,
		instanceRepo:     repositories.NewInstanceRepository(db),
		checkInterval:    10 * time.Second,
		timeout:          5 * time.Second,
		failureThreshold: 3,
	}
}

type HealthCheckConfig struct {
	CheckInterval     time.Duration `json:"check_interval"`
	Timeout           time.Duration `json:"timeout"`
	FailureThreshold  int           `json:"failure_threshold"`
	SuccessThreshold  int           `json:"success_threshold"`
	CheckTypes        []string      `json:"check_types"`
}

type HealthCheckRequest struct {
	InstanceID string            `json:"instance_id" binding:"required"`
	Config     HealthCheckConfig `json:"config"`
}

type HealthCheckResult struct {
	InstanceID     string        `json:"instance_id"`
	CheckType      string        `json:"check_type"`
	Status         string        `json:"status"`
	IsHealthy      bool          `json:"is_healthy"`
	ResponseTime   time.Duration `json:"response_time"`
	ErrorMessage   string        `json:"error_message"`
	Details        CheckDetails  `json:"details"`
	CheckTime      time.Time     `json:"check_time"`
}

type CheckDetails struct {
	TCPReachable    bool   `json:"tcp_reachable"`
	MySQLAlive      bool   `json:"mysql_alive"`
	ReplWorking     bool   `json:"repl_working"`
	SecondsBehind   int    `json:"seconds_behind"`
	ConnectionCount int    `json:"connection_count"`
	QPS             float64 `json:"qps"`
}

type FailureState struct {
	InstanceID      string    `json:"instance_id"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastFailureTime time.Time `json:"last_failure_time"`
	LastSuccessTime time.Time `json:"last_success_time"`
	IsMarkedFailed  bool      `json:"is_marked_failed"`
	FailureReason   string    `json:"failure_reason"`
}

var failureStates = make(map[string]*FailureState)

func (s *HealthCheckService) ExecuteHealthCheck(ctx context.Context, req HealthCheckRequest) (*HealthCheckResult, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	conn, err := s.getInstanceConnection(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance connection: %w", err)
	}

	config := req.Config
	if config.Timeout == 0 {
		config.Timeout = s.timeout
	}
	if config.CheckTypes == nil {
		config.CheckTypes = []string{"tcp", "mysql", "replication"}
	}

	result := &HealthCheckResult{
		InstanceID: req.InstanceID,
		CheckTime:  time.Now(),
		Details:    CheckDetails{},
	}

	for _, checkType := range config.CheckTypes {
		switch checkType {
		case "tcp":
			tcpResult := s.checkTCP(ctx, conn.Host, conn.Port, config.Timeout)
			result.Details.TCPReachable = tcpResult.IsHealthy
			result.ResponseTime = tcpResult.ResponseTime
			if !tcpResult.IsHealthy {
				result.ErrorMessage = tcpResult.ErrorMessage
			}
		case "mysql":
			mysqlResult := s.checkMySQL(ctx, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, config.Timeout)
			result.Details.MySQLAlive = mysqlResult.IsHealthy
			if !mysqlResult.IsHealthy && result.ErrorMessage == "" {
				result.ErrorMessage = mysqlResult.ErrorMessage
			}
			if mysqlResult.IsHealthy {
				result.Details.ConnectionCount = mysqlResult.Details.ConnectionCount
				result.Details.QPS = mysqlResult.Details.QPS
			}
		case "replication":
			if instance.Status.Role == "slave" {
				replResult := s.checkReplication(ctx, conn.Host, conn.Port, conn.Username, conn.PasswordEncrypted, config.Timeout)
				result.Details.ReplWorking = replResult.IsHealthy
				result.Details.SecondsBehind = replResult.Details.SecondsBehind
				if !replResult.IsHealthy && result.ErrorMessage == "" {
					result.ErrorMessage = replResult.ErrorMessage
				}
			}
		}
	}

	result.IsHealthy = result.Details.TCPReachable && result.Details.MySQLAlive
	if instance.Status.Role == "slave" {
		result.IsHealthy = result.IsHealthy && result.Details.ReplWorking
	}

	if result.IsHealthy {
		result.Status = "healthy"
	} else {
		result.Status = "unhealthy"
	}

	s.updateFailureState(req.InstanceID, result.IsHealthy, result.ErrorMessage)

	return result, nil
}

func (s *HealthCheckService) DetectFailure(ctx context.Context, instanceID string) (*FailureState, error) {
	state, exists := failureStates[instanceID]
	if !exists {
		state = &FailureState{
			InstanceID:         instanceID,
			ConsecutiveFailures: 0,
			IsMarkedFailed:     false,
		}
		failureStates[instanceID] = state
	}

	req := HealthCheckRequest{
		InstanceID: instanceID,
		Config: HealthCheckConfig{
			Timeout:    s.timeout,
			CheckTypes: []string{"tcp", "mysql"},
		},
	}

	result, err := s.ExecuteHealthCheck(ctx, req)
	if err != nil {
		state.ConsecutiveFailures++
		state.LastFailureTime = time.Now()
		state.FailureReason = err.Error()
		return state, err
	}

	if !result.IsHealthy {
		state.ConsecutiveFailures++
		state.LastFailureTime = time.Now()
		state.FailureReason = result.ErrorMessage
	} else {
		state.ConsecutiveFailures = 0
		state.LastSuccessTime = time.Now()
		state.FailureReason = ""
	}

	if state.ConsecutiveFailures >= s.failureThreshold && !state.IsMarkedFailed {
		state.IsMarkedFailed = true
	}

	return state, nil
}

func (s *HealthCheckService) GetFailureState(instanceID string) *FailureState {
	return failureStates[instanceID]
}

func (s *HealthCheckService) ClearFailureState(instanceID string) {
	delete(failureStates, instanceID)
}

func (s *HealthCheckService) StartPeriodicCheck(ctx context.Context, instanceID string, interval time.Duration) error {
	if interval == 0 {
		interval = s.checkInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, _ = s.DetectFailure(ctx, instanceID)
		}
	}
}

func (s *HealthCheckService) checkTCP(ctx context.Context, host string, port int, timeout time.Duration) *HealthCheckResult {
	startTime := time.Now()

	_, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "tcp",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("TCP connection failed: %v", err),
		}
	}
	conn.Close()

	return &HealthCheckResult{
		CheckType:    "tcp",
		IsHealthy:    true,
		ResponseTime: time.Since(startTime),
	}
}

func (s *HealthCheckService) checkMySQL(ctx context.Context, host string, port int, username, password string, timeout time.Duration) *HealthCheckResult {
	startTime := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=%ds", username, password, net.JoinHostPort(host, fmt.Sprintf("%d", port)), int(timeout.Seconds()))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "mysql",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to open MySQL connection: %v", err),
		}
	}
	defer db.Close()

	err = db.PingContext(checkCtx)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "mysql",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("MySQL ping failed: %v", err),
		}
	}

	details := CheckDetails{}
	row := db.QueryRowContext(checkCtx, "SHOW STATUS WHERE Variable_name IN ('Threads_connected', 'Questions')")
	var varName, value string
	for {
		err := row.Scan(&varName, &value)
		if err != nil {
			break
		}
		switch varName {
		case "Threads_connected":
			fmt.Sscanf(value, "%d", &details.ConnectionCount)
		case "Questions":
			fmt.Sscanf(value, "%f", &details.QPS)
		}
	}

	return &HealthCheckResult{
		CheckType:    "mysql",
		IsHealthy:    true,
		ResponseTime: time.Since(startTime),
		Details:      details,
	}
}

func (s *HealthCheckService) checkReplication(ctx context.Context, host string, port int, username, password string, timeout time.Duration) *HealthCheckResult {
	startTime := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=%ds", username, password, net.JoinHostPort(host, fmt.Sprintf("%d", port)), int(timeout.Seconds()))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to connect for replication check: %v", err),
		}
	}
	defer db.Close()

	err = db.PingContext(checkCtx)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("MySQL ping failed: %v", err),
		}
	}

	details := CheckDetails{}
	row := db.QueryRowContext(checkCtx, "SHOW SLAVE STATUS")
	var slaveIORunning, slaveSQLRunning string
	var secondsBehind sql.NullInt64

	err = row.Scan(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &slaveIORunning, &slaveSQLRunning, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &secondsBehind)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to get slave status: %v", err),
		}
	}

	isHealthy := slaveIORunning == "Yes" && slaveSQLRunning == "Yes"
	if secondsBehind.Valid {
		details.SecondsBehind = int(secondsBehind.Int64)
	}

	if !isHealthy {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Replication not running: IO=%s, SQL=%s", slaveIORunning, slaveSQLRunning),
			Details:      details,
		}
	}

	return &HealthCheckResult{
		CheckType:    "replication",
		IsHealthy:    true,
		ResponseTime: time.Since(startTime),
		Details:      details,
	}
}

func (s *HealthCheckService) getInstanceConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	query := `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled
		FROM instance_connections WHERE instance_id = ?
	`

	conn := &models.InstanceConnection{}
	err := s.db.Pool.QueryRowContext(ctx, query, instanceID).Scan(
		&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled)

	if err != nil {
		return nil, fmt.Errorf("failed to get instance connection: %w", err)
	}

	return conn, nil
}

func (s *HealthCheckService) updateFailureState(instanceID string, isHealthy bool, errorMsg string) {
	state, exists := failureStates[instanceID]
	if !exists {
		state = &FailureState{
			InstanceID: instanceID,
		}
		failureStates[instanceID] = state
	}

	if isHealthy {
		state.ConsecutiveFailures = 0
		state.LastSuccessTime = time.Now()
		state.IsMarkedFailed = false
		state.FailureReason = ""
	} else {
		state.ConsecutiveFailures++
		state.LastFailureTime = time.Now()
		state.FailureReason = errorMsg

		if state.ConsecutiveFailures >= s.failureThreshold {
			state.IsMarkedFailed = true
		}
	}
}

func (s *HealthCheckService) BatchHealthCheck(ctx context.Context, instanceIDs []string) ([]HealthCheckResult, error) {
	results := make([]HealthCheckResult, 0, len(instanceIDs))

	for _, instanceID := range instanceIDs {
		req := HealthCheckRequest{
			InstanceID: instanceID,
		}
		result, err := s.ExecuteHealthCheck(ctx, req)
		if err != nil {
			result = &HealthCheckResult{
				InstanceID:   instanceID,
				Status:       "error",
				IsHealthy:    false,
				ErrorMessage: err.Error(),
				CheckTime:    time.Now(),
			}
		}
		results = append(results, *result)
	}

	return results, nil
}