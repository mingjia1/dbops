package services

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"golang.org/x/sync/errgroup"
)

type HealthCheckService struct {
	db               *repositories.Database
	instanceRepo     *repositories.InstanceRepository
	checkInterval    time.Duration
	timeout          time.Duration
	failureThreshold int
	encryptionKey    string
	// B9: failureStates 并发安全 - 从 package var 改为字段 + mutex
	failureMu     sync.Mutex
	failureStates map[string]*FailureState
}

func NewHealthCheckService(db *repositories.Database, encryptionKey string) *HealthCheckService {
	return &HealthCheckService{
		db:               db,
		instanceRepo:     repositories.NewInstanceRepository(db),
		checkInterval:    10 * time.Second,
		timeout:          5 * time.Second,
		failureThreshold: 3,
		encryptionKey:    encryptionKey,
		failureStates:    make(map[string]*FailureState),
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

// failureStates moved into HealthCheckService.failureStates (B9: was package var with no lock)

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
	s.failureMu.Lock()
	state, exists := s.failureStates[instanceID]
	if !exists {
		state = &FailureState{
			InstanceID:          instanceID,
			ConsecutiveFailures: 0,
			IsMarkedFailed:      false,
		}
		s.failureStates[instanceID] = state
	}
	s.failureMu.Unlock()

	req := HealthCheckRequest{
		InstanceID: instanceID,
		Config: HealthCheckConfig{
			Timeout:    s.timeout,
			CheckTypes: []string{"tcp", "mysql"},
		},
	}

	result, err := s.ExecuteHealthCheck(ctx, req)

	s.failureMu.Lock()
	defer s.failureMu.Unlock()
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
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	return s.failureStates[instanceID]
}

func (s *HealthCheckService) ClearFailureState(instanceID string) {
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	delete(s.failureStates, instanceID)
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

	// B9: 用 Dialer.DialContext 让 ctx cancel 能真正中断握手, 不再"造个 ctx 立刻 cancel".
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "tcp",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("TCP connection failed: %v", err),
		}
	}
	_ = conn.Close()

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

	// A5: PasswordEncrypted 是 AES-GCM 密文, 必须先 Decrypt, 否则 MySQL 用密文当密码必败.
	plain, err := utils.Decrypt(password, s.encryptionKey)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "mysql",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to decrypt password: %v", err),
		}
	}

	// B1: 用 mysql.Config.FormatDSN() 自动转义 @ / : 等 DSN 特殊字符, 杜绝 Sprintf 拼接注入.
	cfg := mysql.NewConfig()
	cfg.User = username
	cfg.Passwd = plain
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = timeout
	cfg.ReadTimeout = timeout
	cfg.WriteTimeout = timeout
	dsn := cfg.FormatDSN()

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

	// A5: 先 Decrypt, 同 checkMySQL.
	plain, err := utils.Decrypt(password, s.encryptionKey)
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to decrypt password: %v", err),
		}
	}

	// B1: 用 mysql.Config.FormatDSN(), 同 checkMySQL.
	cfg := mysql.NewConfig()
	cfg.User = username
	cfg.Passwd = plain
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = timeout
	cfg.ReadTimeout = timeout
	cfg.WriteTimeout = timeout
	dsn := cfg.FormatDSN()

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

	// Use SHOW SLAVE STATUS with column-name based scanning
	rows, err := db.QueryContext(checkCtx, "SHOW SLAVE STATUS")
	if err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to get slave status: %v", err),
		}
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if !rows.Next() {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: "No slave status found (not a replica?)",
		}
	}

	vals := make([]interface{}, len(cols))
	for i := range vals {
		vals[i] = new(sql.NullString)
	}
	if err := rows.Scan(vals...); err != nil {
		return &HealthCheckResult{
			CheckType:    "replication",
			IsHealthy:    false,
			ResponseTime: time.Since(startTime),
			ErrorMessage: fmt.Sprintf("Failed to scan slave status: %v", err),
		}
	}

	slaveIORunning := ""
	slaveSQLRunning := ""
	var secondsBehind sql.NullInt64

	colMap := make(map[string]string, len(cols))
	for i, col := range cols {
		if ns, ok := vals[i].(*sql.NullString); ok && ns.Valid {
			colMap[col] = ns.String
		}
	}

	if v, ok := colMap["Slave_IO_Running"]; ok {
		slaveIORunning = v
	}
	if v, ok := colMap["Slave_SQL_Running"]; ok {
		slaveSQLRunning = v
	}
	if v, ok := colMap["Seconds_Behind_Master"]; ok {
		if n, err := parseInt64(v); err == nil {
			secondsBehind = sql.NullInt64{Int64: n, Valid: true}
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
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	state, exists := s.failureStates[instanceID]
	if !exists {
		state = &FailureState{
			InstanceID: instanceID,
		}
		s.failureStates[instanceID] = state
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
	results := make([]HealthCheckResult, len(instanceIDs))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5) // max 5 concurrent health checks

	for i, instanceID := range instanceIDs {
		i, instanceID := i, instanceID
		g.Go(func() error {
			req := HealthCheckRequest{
				InstanceID: instanceID,
				Config: HealthCheckConfig{
					CheckTypes: batchHealthCheckTypes(),
				},
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
			mu.Lock()
			results[i] = *result
			mu.Unlock()
			return nil // don't fail entire batch on individual errors
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}

func batchHealthCheckTypes() []string {
	return []string{"tcp", "mysql"}
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
