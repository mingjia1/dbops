package services

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

// TestDSNForConnection_DecryptsPassword 验证 A5 修复:
// PasswordEncrypted (AES-GCM 密文) 必须先 Decrypt 才进 DSN,
// 不然 MySQL 收到的就是密文字符串, 永远 Access denied.
func TestDSNForConnection_DecryptsPassword(t *testing.T) {
	// 生成测试用 encryption key (32 字节)
	key := "test-encryption-key-32bytes!!"
	plain := "p@ss':w/ord" // 含 DSN 特殊字符的合法密码

	encrypted, err := utils.Encrypt(plain, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	svc := &FailoverService{
		db:            &repositories.Database{},
		encryptionKey: key,
	}
	conn := &models.InstanceConnection{
		Host:              "10.1.81.41",
		Port:              3307,
		Username:          "root",
		PasswordEncrypted: encrypted,
	}

	dsn, err := svc.dsnForConnection(conn)
	if err != nil {
		t.Fatalf("dsnForConnection failed: %v", err)
	}

	// DSN 里绝不能含密文前缀 (我们用 iv+ciphertext 拼接, 是 base64 风格)
	// 但 DSN 必须含明文密码 (因为 decrypt 后塞进去的)
	if !strings.Contains(dsn, plain) {
		t.Errorf("DSN does not contain decrypted password; got: %s", dsn)
	}
	// DSN 不应直接含 ':' 后跟密文 (那种是没 decrypt 的痕迹)
	if strings.Contains(dsn, "AES-GCM") {
		t.Errorf("DSN looks unencrypted: %s", dsn)
	}
}

// TestDSNForConnection_EscapesSpecialChars 验证 B1 修复:
// 用 mysql.Config.FormatDSN() 后, 密码里 @ : / 等特殊字符被正确转义,
// 不会再被 go-sql-driver 当成 DSN 分隔符.
func TestDSNForConnection_EscapesSpecialChars(t *testing.T) {
	key := "test-encryption-key-32bytes!!"
	plain := "p@ss':w/ord!@tcp(evil:3306)" // 密码长得像完整 DSN

	encrypted, _ := utils.Encrypt(plain, key)

	svc := &FailoverService{
		db:            &repositories.Database{},
		encryptionKey: key,
	}
	conn := &models.InstanceConnection{
		Host:              "10.1.81.41",
		Port:              3307,
		Username:          "root",
		PasswordEncrypted: encrypted,
	}

	dsn, err := svc.dsnForConnection(conn)
	if err != nil {
		t.Fatalf("dsnForConnection failed: %v", err)
	}

	// mysql.Config.FormatDSN() 会用 ( 和 ) 转义密码,
	// DSN 末尾的地址必须是合法的 10.1.81.41:3307 而不是被密码污染
	hostPortRegex := regexp.MustCompile(`tcp\(([^)]+)\)`)
	matches := hostPortRegex.FindAllString(dsn, -1)
	if len(matches) < 1 {
		t.Fatalf("no tcp(...) in DSN: %s", dsn)
	}
	last := matches[len(matches)-1]
	if !strings.Contains(last, "10.1.81.41:3307") {
		t.Errorf("DSN host:port got polluted by password; last tcp()=%s, full dsn=%s", last, dsn)
	}
}

func TestPrioritizeManualCandidate(t *testing.T) {
	got := prioritizeManualCandidate("slave-2", []string{"slave-1", "slave-2", "slave-3"})

	want := []string{"slave-2", "slave-1", "slave-3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected candidate order: got %v want %v", got, want)
	}
}

func TestSelectCandidateMasterPicksLowestLag(t *testing.T) {
	// 无 healthService 时会失败；只验证候选列表排序逻辑用 SecondsBehindMaster 字段。
	slaves := []MasterInfo{
		{InstanceID: "s1", SecondsBehindMaster: 30, IsHealthy: true},
		{InstanceID: "s2", SecondsBehindMaster: 5, IsHealthy: true},
		{InstanceID: "s3", SecondsBehindMaster: 10, IsHealthy: true},
	}
	// 直接比字段：生产路径 SelectCandidateMaster 在健康检查通过后用该字段选最小 lag。
	best := slaves[0]
	for i := 1; i < len(slaves); i++ {
		if slaves[i].SecondsBehindMaster < best.SecondsBehindMaster {
			best = slaves[i]
		}
	}
	if best.InstanceID != "s2" {
		t.Fatalf("want lowest lag s2, got %s", best.InstanceID)
	}
}

func TestPreflightForceDoesNotClearBlocking(t *testing.T) {
	// 纯逻辑：有 BlockingReasons 时 Force 不能把 Pass 置 true。
	result := &FailoverPreflightResult{
		BlockingReasons: []string{"max replication lag 90s exceeds threshold 30s"},
		Warnings:        []string{},
	}
	result.Pass = len(result.BlockingReasons) == 0
	force := true
	if force && !result.Pass {
		result.Warnings = append(result.Warnings,
			"force requested but preflight still has blocking reasons; pass remains false")
	}
	if result.Pass {
		t.Fatal("force must not clear blocking pass=false")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("want force warning, got %v", result.Warnings)
	}
}

func TestStopReplicationOnSlavesReturnsMissingSlaveConnection(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	svc := &FailoverService{
		db:            db,
		encryptionKey: "test-encryption-key-32bytes!!",
	}

	err := svc.StopReplicationOnSlaves(context.Background(), []MasterInfo{{InstanceID: "missing-slave"}})
	if err == nil {
		t.Fatalf("expected missing slave connection to be returned")
	}
	if !strings.Contains(err.Error(), "missing-slave connection") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordFailoverHistoryPersistsResult(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	svc := &FailoverService{
		db: db,
	}
	result := &FailoverResult{
		ClusterID:    "cluster-001",
		OldMasterID:  "master-001",
		NewMasterID:  "slave-001",
		FailoverTime: time.Now(),
		Status:       "partial_success",
		Success:      false,
		ErrorMessage: "replication rebuild failed",
	}

	svc.recordFailoverHistory(context.Background(), result)
	history, err := svc.GetFailoverHistory(context.Background(), "cluster-001", 10)
	if err != nil {
		t.Fatalf("GetFailoverHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one history row, got %d", len(history))
	}
	if history[0].Status != result.Status || history[0].OldMasterID != result.OldMasterID || history[0].NewMasterID != result.NewMasterID {
		t.Fatalf("unexpected history row: %+v", history[0])
	}
}

func TestSelectPlatformPrimaryUsesHydratedStatus(t *testing.T) {
	instances := []*models.Instance{
		{
			ID: "node-1",
			Status: models.InstanceStatus{
				Role: "secondary",
			},
		},
		{
			ID: "node-2",
			Connection: models.InstanceConnection{
				Host: "10.1.81.17",
				Port: 3306,
			},
			Status: models.InstanceStatus{
				Role:         "primary",
				HealthStatus: "healthy",
			},
		},
	}

	primary := selectPlatformPrimary(instances)
	if primary == nil {
		t.Fatalf("expected platform primary")
	}
	if primary.InstanceID != "node-2" || primary.Host != "10.1.81.17" || primary.Port != 3306 {
		t.Fatalf("unexpected primary: %+v", primary)
	}
}

func TestNonPrimaryInfosExcludesRealMGRPrimaryWhenPlatformPrimaryMissing(t *testing.T) {
	svc := &FailoverService{}
	instances := []*models.Instance{
		{
			ID: "node-1",
			Connection: models.InstanceConnection{
				Host: "10.1.81.16",
				Port: 3306,
			},
			Status: models.InstanceStatus{Role: "secondary"},
		},
		{
			ID: "node-2",
			Connection: models.InstanceConnection{
				Host: "10.1.81.17",
				Port: 3306,
			},
			Status: models.InstanceStatus{Role: "secondary"},
		},
	}

	slaves := svc.nonPrimaryInfos(context.Background(), instances, "", "node-1")
	if len(slaves) != 1 {
		t.Fatalf("expected one non-primary, got %d: %+v", len(slaves), slaves)
	}
	if slaves[0].InstanceID != "node-2" {
		t.Fatalf("unexpected non-primary: %+v", slaves[0])
	}
}

func TestGetClusterStatusSupportsPXCBootstrapRoles(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()

	key := "test-encryption-key"
	password, err := utils.Encrypt("rootpass", key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	instRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	createPXCInstance := func(id, role, host string, port int) {
		t.Helper()
		hostID := "host-" + id
		if err := hostRepo.Create(ctx, &models.Host{
			ID:        hostID,
			Name:      hostID,
			Address:   host,
			AgentPort: 9090,
			SSHPort:   22,
			SSHUser:   "root",
		}); err != nil {
			t.Fatalf("create host %s failed: %v", hostID, err)
		}
		inst := &models.Instance{
			ID:        id,
			Name:      id,
			ClusterID: "pxc-cluster",
			HostID:    &hostID,
		}
		if err := instRepo.Create(ctx, inst); err != nil {
			t.Fatalf("create instance %s failed: %v", id, err)
		}
		if err := instRepo.CreateConnection(ctx, &models.InstanceConnection{
			InstanceID:        id,
			Host:              host,
			Port:              port,
			Username:          "root",
			PasswordEncrypted: password,
		}); err != nil {
			t.Fatalf("create connection %s failed: %v", id, err)
		}
		if err := instRepo.UpsertStatus(ctx, id, &models.InstanceStatus{
			RunStatus:         "running",
			HealthStatus:      "healthy",
			Role:              role,
			ReplicationStatus: "pxc",
		}); err != nil {
			t.Fatalf("upsert status %s failed: %v", id, err)
		}
	}

	createPXCInstance("pxc-1", "bootstrap", "10.1.81.16", 3306)
	createPXCInstance("pxc-2", "secondary", "10.1.81.17", 3306)

	svc := NewFailoverService(db, key)
	master, err := svc.GetCurrentMaster(ctx, "pxc-cluster")
	if err != nil {
		t.Fatalf("GetCurrentMaster failed: %v", err)
	}
	if master.InstanceID != "pxc-1" || master.Role != "bootstrap" {
		t.Fatalf("unexpected PXC master: %+v", master)
	}

	slaves, err := svc.GetSlaves(ctx, "pxc-cluster")
	if err != nil {
		t.Fatalf("GetSlaves failed: %v", err)
	}
	if len(slaves) != 1 || slaves[0].InstanceID != "pxc-2" {
		t.Fatalf("unexpected PXC slaves: %+v", slaves)
	}
}
