package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountManager_EmptyPassword(t *testing.T) {
	m := NewAccountManager()

	err := m.SetupRootAccount(nil, "localhost", 3306, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_EmptyReplUser(t *testing.T) {
	m := NewAccountManager()

	err := m.SetupReplAccount(nil, "localhost", 3306, "root", "pass", "", "replpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_EmptyReplPass(t *testing.T) {
	m := NewAccountManager()

	err := m.SetupReplAccount(nil, "localhost", 3306, "root", "pass", "repl", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_EmptyMonitorUser(t *testing.T) {
	m := NewAccountManager()

	err := m.SetupMonitorAccount(nil, "localhost", 3306, "root", "pass", "", "monpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_EmptyMonitorPass(t *testing.T) {
	m := NewAccountManager()

	err := m.SetupMonitorAccount(nil, "localhost", 3306, "root", "pass", "monitor", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_RotatePassword_EmptyTarget(t *testing.T) {
	m := NewAccountManager()

	err := m.RotatePassword(nil, "localhost", 3306, "root", "pass", "", "newpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestAccountManager_RotatePassword_EmptyPass(t *testing.T) {
	m := NewAccountManager()

	err := m.RotatePassword(nil, "localhost", 3306, "root", "pass", "repl", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}
