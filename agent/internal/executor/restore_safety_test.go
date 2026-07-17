package executor

import "testing"

func TestAssertSafeRestoreDatadir(t *testing.T) {
	if err := assertSafeRestoreDatadir(""); err == nil {
		t.Fatal("empty datadir must fail")
	}
	if err := assertSafeRestoreDatadir("/"); err == nil {
		t.Fatal("root datadir must fail")
	}
	if err := assertSafeRestoreDatadir("/var"); err == nil {
		t.Fatal("too broad /var must fail")
	}
	if err := assertSafeRestoreDatadir("/var/lib/mysql"); err != nil {
		t.Fatalf("normal datadir should pass: %v", err)
	}
}
