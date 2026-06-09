package services

import (
	"regexp"
	"strings"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
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
