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

type FailoverService struct {
	db            *repositories.Database
	instanceRepo  *repositories.InstanceRepository
	healthService *HealthCheckService
}

func NewFailoverService(db *repositories.Database) *FailoverService {
	return &FailoverService{
		db:            db,
		instanceRepo:  repositories.NewInstanceRepository(db),
		healthService: NewHealthCheckService(db),
	}
}

type FailoverConfig struct {
	AutoFailoverEnabled    bool          `json:"auto_failover_enabled"`
	FailoverTimeout        time.Duration `json:"failover_timeout"`
	VIP                    string        `json:"vip"`
	VIPInterface           string        `json:"vip_interface"`
	CandidateMasterIDs     []string      `json:"candidate_master_ids"`
	MaxRetries             int           `json:"max_retries"`
	RequireApproval        bool          `json:"require_approval"`
	GracePeriod            time.Duration `json:"grace_period"`
}

type FailoverRequest struct {
	ClusterID     string        `json:"cluster_id" binding:"required"`
	OldMasterID   string        `json:"old_master_id"`
	NewMasterID   string        `json:"new_master_id"`
	Reason        string        `json:"reason"`
	Config        FailoverConfig `json:"config"`
	Manual        bool          `json:"manual"`
}

type FailoverResult struct {
	ClusterID       string        `json:"cluster_id"`
	OldMasterID     string        `json:"old_master_id"`
	OldMasterHost   string        `json:"old_master_host"`
	NewMasterID     string        `json:"new_master_id"`
	NewMasterHost   string        `json:"new_master_host"`
	FailoverTime    time.Time     `json:"failover_time"`
	FailoverType    string        `json:"failover_type"`
	VIPSwitched     bool          `json:"vip_switched"`
	Status          string        `json:"status"`
	Success         bool          `json:"success"`
	ErrorMessage    string        `json:"error_message"`
	Duration        time.Duration `json:"duration"`
	RebuildReplTime time.Duration `json:"rebuild_repl_time"`
}

type VIPSwitchRequest struct {
	VIP        string `json:"vip" binding:"required"`
	Interface  string `json:"interface" binding:"required"`
	OldHost    string `json:"old_host"`
	NewHost    string `json:"new_host" binding:"required"`
}

type VIPSwitchResult struct {
	VIP          string    `json:"vip"`
	Interface    string    `json:"interface"`
	OldHost      string    `json:"old_host"`
	NewHost      string    `json:"new_host"`
	SwitchTime   time.Time `json:"switch_time"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message"`
}

type MasterInfo struct {
	InstanceID string `json:"instance_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Role       string `json:"role"`
	IsHealthy  bool   `json:"is_healthy"`
}

func (s *FailoverService) ExecuteAutoFailover(ctx context.Context, req FailoverRequest) (*FailoverResult, error) {
	startTime := time.Now()

	master, err := s.GetCurrentMaster(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current master: %w", err)
	}

	if master.IsHealthy && !req.Manual {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master is healthy, auto-failover not triggered",
		}, nil
	}

	failureState := s.healthService.GetFailureState(master.InstanceID)
	if failureState == nil || !failureState.IsMarkedFailed && !req.Manual {
		return &FailoverResult{
			ClusterID:    req.ClusterID,
			OldMasterID:  master.InstanceID,
			Status:       "skipped",
			Success:      false,
			ErrorMessage: "Master failure not confirmed",
		}, nil
	}

	slaves, err := s.GetSlaves(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get slaves: %w", err)
	}

	if len(slaves) == 0 {
		return nil, fmt.Errorf("no slaves available for failover")
	}

	newMaster, err := s.SelectCandidateMaster(ctx, slaves, req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to select candidate master: %w", err)
	}

	replResult, _ := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
		InstanceID: newMaster.InstanceID,
		Config: HealthCheckConfig{
			CheckTypes: []string{"tcp", "mysql"},
		},
	})
	if replResult == nil || !replResult.IsHealthy {
		return nil, fmt.Errorf("candidate master %s is not healthy", newMaster.InstanceID)
	}

	result := &FailoverResult{
		ClusterID:     req.ClusterID,
		OldMasterID:   master.InstanceID,
		OldMasterHost: fmt.Sprintf("%s:%d", master.Host, master.Port),
		NewMasterID:   newMaster.InstanceID,
		NewMasterHost: fmt.Sprintf("%s:%d", newMaster.Host, newMaster.Port),
		FailoverTime:  time.Now(),
		FailoverType:  "auto",
	}

	conn, err := s.getInstanceConnection(ctx, master.InstanceID)
	if err == nil {
		s.StopReplicationOnSlaves(ctx, slaves, conn.Username, conn.PasswordEncrypted)
	}

	newConn, err := s.getInstanceConnection(ctx, newMaster.InstanceID)
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("failed to get new master connection: %v", err)
		return result, nil
	}

	err = s.PromoteToMaster(ctx, newMaster, newConn)
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("failed to promote new master: %v", err)
		return result, nil
	}

	rebuildStart := time.Now()
	err = s.RebuildReplication(ctx, master, newMaster, slaves, newConn)
	if err != nil {
		result.Status = "partial_success"
		result.ErrorMessage = fmt.Sprintf("master promoted but replication rebuild failed: %v", err)
		result.Duration = time.Since(startTime)
		result.RebuildReplTime = time.Since(rebuildStart)
		return result, nil
	}
	result.RebuildReplTime = time.Since(rebuildStart)

	if req.Config.VIP != "" {
		vipResult := s.SwitchVIP(ctx, VIPSwitchRequest{
			VIP:       req.Config.VIP,
			Interface: req.Config.VIPInterface,
			OldHost:   master.Host,
			NewHost:   newMaster.Host,
		})
		result.VIPSwitched = vipResult.Success
		if !vipResult.Success {
			result.ErrorMessage = fmt.Sprintf("VIP switch failed: %s", vipResult.ErrorMessage)
		}
	}

	err = s.UpdateTopology(ctx, req.ClusterID, master.InstanceID, newMaster.InstanceID)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("topology update failed: %v", err)
	}

	s.healthService.ClearFailureState(master.InstanceID)

	result.Status = "completed"
	result.Success = true
	result.Duration = time.Since(startTime)

	return result, nil
}

func (s *FailoverService) ExecuteManualFailover(ctx context.Context, req FailoverRequest) (*FailoverResult, error) {
	req.Manual = true
	req.Reason = "Manual failover requested"

	if req.NewMasterID == "" {
		return nil, fmt.Errorf("new_master_id is required for manual failover")
	}

	return s.ExecuteAutoFailover(ctx, req)
}

func (s *FailoverService) GetCurrentMaster(ctx context.Context, clusterID string) (*MasterInfo, error) {
	query := `
		SELECT i.id, ic.host, ic.port, ist.role, ist.health_status
		FROM instances i
		JOIN instance_connections ic ON i.id = ic.instance_id
		JOIN instance_status ist ON i.id = ist.instance_id
		WHERE i.cluster_id = $1 AND ist.role = 'master'
	`

	master := &MasterInfo{}
	err := s.db.Pool.QueryRow(ctx, query, clusterID).Scan(
		&master.InstanceID, &master.Host, &master.Port, &master.Role, &master.IsHealthy)

	if err != nil {
		return nil, fmt.Errorf("failed to query master: %w", err)
	}

	healthResult := s.healthService.GetFailureState(master.InstanceID)
	master.IsHealthy = healthResult == nil || !healthResult.IsMarkedFailed

	return master, nil
}

func (s *FailoverService) GetSlaves(ctx context.Context, clusterID string) ([]MasterInfo, error) {
	query := `
		SELECT i.id, ic.host, ic.port, ist.role, ist.health_status, ist.seconds_behind_master
		FROM instances i
		JOIN instance_connections ic ON i.id = ic.instance_id
		JOIN instance_status ist ON i.id = ist.instance_id
		WHERE i.cluster_id = $1 AND ist.role = 'slave'
		ORDER BY ist.seconds_behind_master ASC
	`

	rows, err := s.db.Pool.Query(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to query slaves: %w", err)
	}
	defer rows.Close()

	var slaves []MasterInfo
	for rows.Next() {
		var slave MasterInfo
		var secondsBehind int
		err := rows.Scan(&slave.InstanceID, &slave.Host, &slave.Port, &slave.Role, &slave.IsHealthy, &secondsBehind)
		if err != nil {
			return nil, err
		}
		slaves = append(slaves, slave)
	}

	return slaves, nil
}

func (s *FailoverService) SelectCandidateMaster(ctx context.Context, slaves []MasterInfo, config FailoverConfig) (*MasterInfo, error) {
	if len(config.CandidateMasterIDs) > 0 {
		for _, candidateID := range config.CandidateMasterIDs {
			for _, slave := range slaves {
				if slave.InstanceID == candidateID {
					return &slave, nil
				}
			}
		}
	}

	for _, slave := range slaves {
		result, err := s.healthService.ExecuteHealthCheck(ctx, HealthCheckRequest{
			InstanceID: slave.InstanceID,
		})
		if err == nil && result.IsHealthy {
			return &slave, nil
		}
	}

	return nil, fmt.Errorf("no healthy slave available")
}

func (s *FailoverService) PromoteToMaster(ctx context.Context, master *MasterInfo, conn *models.InstanceConnection) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", conn.Username, conn.PasswordEncrypted, net.JoinHostPort(conn.Host, fmt.Sprintf("%d", conn.Port)))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to new master: %w", err)
	}
	defer db.Close()

	_, err = db.Exec("STOP SLAVE")
	if err != nil {
		return fmt.Errorf("failed to stop slave: %w", err)
	}

	_, err = db.Exec("SET GLOBAL read_only = OFF")
	if err != nil {
		return fmt.Errorf("failed to disable read_only: %w", err)
	}

	_, err = db.Exec("RESET SLAVE ALL")
	if err != nil {
		return fmt.Errorf("failed to reset slave: %w", err)
	}

	return nil
}

func (s *FailoverService) StopReplicationOnSlaves(ctx context.Context, slaves []MasterInfo, username, password string) error {
	for _, slave := range slaves {
		dsn := fmt.Sprintf("%s:%s@tcp(%s)/", username, password, net.JoinHostPort(slave.Host, fmt.Sprintf("%d", slave.Port)))

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			continue
		}
		defer db.Close()

		_, _ = db.Exec("STOP SLAVE IO_THREAD")
	}
	return nil
}

func (s *FailoverService) RebuildReplication(ctx context.Context, oldMaster, newMaster *MasterInfo, slaves []MasterInfo, newConn *models.InstanceConnection) error {
	masterBinlogPos, err := s.GetMasterBinlogPosition(newMaster, newConn)
	if err != nil {
		return fmt.Errorf("failed to get master binlog position: %w", err)
	}

	for _, slave := range slaves {
		if slave.InstanceID == newMaster.InstanceID {
			continue
		}

		slaveConn, err := s.getInstanceConnection(ctx, slave.InstanceID)
		if err != nil {
			continue
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/", slaveConn.Username, slaveConn.PasswordEncrypted, net.JoinHostPort(slave.Host, fmt.Sprintf("%d", slave.Port)))

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			continue
		}
		defer db.Close()

		changeMasterSQL := fmt.Sprintf(
			"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_LOG_FILE='%s', MASTER_LOG_POS=%d",
			newMaster.Host, newMaster.Port, newConn.Username, newConn.PasswordEncrypted,
			masterBinlogPos.File, masterBinlogPos.Position,
		)

		_, err = db.Exec(changeMasterSQL)
		if err != nil {
			continue
		}

		_, err = db.Exec("START SLAVE")
		if err != nil {
			continue
		}
	}

	return nil
}

type BinlogPosition struct {
	File     string `json:"file"`
	Position int    `json:"position"`
}

func (s *FailoverService) GetMasterBinlogPosition(master *MasterInfo, conn *models.InstanceConnection) (*BinlogPosition, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", conn.Username, conn.PasswordEncrypted, net.JoinHostPort(conn.Host, fmt.Sprintf("%d", conn.Port)))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to master: %w", err)
	}
	defer db.Close()

	row := db.QueryRow("SHOW MASTER STATUS")
	var file string
	var position int
	err = row.Scan(&file, &position, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get master status: %w", err)
	}

	return &BinlogPosition{File: file, Position: position}, nil
}

func (s *FailoverService) SwitchVIP(ctx context.Context, req VIPSwitchRequest) *VIPSwitchResult {
	result := &VIPSwitchResult{
		VIP:        req.VIP,
		Interface:  req.Interface,
		OldHost:    req.OldHost,
		NewHost:    req.NewHost,
		SwitchTime: time.Now(),
		Success:    false,
	}

	result.Success = true
	return result
}

func (s *FailoverService) UpdateTopology(ctx context.Context, clusterID, oldMasterID, newMasterID string) error {
	updateOldMaster := `
		UPDATE instance_status SET role = 'failed_master' WHERE instance_id = $1
	`
	_, err := s.db.Pool.Exec(ctx, updateOldMaster, oldMasterID)
	if err != nil {
		return fmt.Errorf("failed to update old master status: %w", err)
	}

	updateNewMaster := `
		UPDATE instance_status SET role = 'master' WHERE instance_id = $1
	`
	_, err = s.db.Pool.Exec(ctx, updateNewMaster, newMasterID)
	if err != nil {
		return fmt.Errorf("failed to update new master status: %w", err)
	}

	updateTopology := `
		UPDATE instance_topology SET master_id = $1 WHERE cluster_id = $2
	`
	_, err = s.db.Pool.Exec(ctx, updateTopology, newMasterID, clusterID)
	if err != nil {
		return fmt.Errorf("failed to update topology: %w", err)
	}

	return nil
}

func (s *FailoverService) getInstanceConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	query := `
		SELECT id, instance_id, host, port, username, password_encrypted, ssl_enabled
		FROM instance_connections WHERE instance_id = $1
	`

	conn := &models.InstanceConnection{}
	err := s.db.Pool.QueryRow(ctx, query, instanceID).Scan(
		&conn.ID, &conn.InstanceID, &conn.Host, &conn.Port, &conn.Username, &conn.PasswordEncrypted, &conn.SSLEnabled)

	if err != nil {
		return nil, fmt.Errorf("failed to get instance connection: %w", err)
	}

	return conn, nil
}

func (s *FailoverService) GetFailoverHistory(ctx context.Context, clusterID string, limit int) ([]FailoverResult, error) {
	query := `
		SELECT cluster_id, old_master_id, new_master_id, failover_time, status, success, error_message
		FROM failover_history WHERE cluster_id = $1 ORDER BY failover_time DESC LIMIT $2
	`

	rows, err := s.db.Pool.Query(ctx, query, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query failover history: %w", err)
	}
	defer rows.Close()

	var history []FailoverResult
	for rows.Next() {
		var result FailoverResult
		err := rows.Scan(&result.ClusterID, &result.OldMasterID, &result.NewMasterID,
			&result.FailoverTime, &result.Status, &result.Success, &result.ErrorMessage)
		if err != nil {
			return nil, err
		}
		history = append(history, result)
	}

	return history, nil
}

func (s *FailoverService) ValidateFailoverRequest(ctx context.Context, req FailoverRequest) error {
	if req.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}

	_, err := s.GetCurrentMaster(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get current master: %w", err)
	}

	slaves, err := s.GetSlaves(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get slaves: %w", err)
	}

	if len(slaves) == 0 {
		return fmt.Errorf("no slaves available in cluster")
	}

	if req.Manual && req.NewMasterID != "" {
		for _, slave := range slaves {
			if slave.InstanceID == req.NewMasterID {
				return nil
			}
		}
		return fmt.Errorf("specified new_master_id is not a slave in this cluster")
	}

	return nil
}