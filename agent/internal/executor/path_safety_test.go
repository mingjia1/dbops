package executor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeJoin_NormalPath(t *testing.T) {
	path, err := SafeJoin("/data", "mysql_3306")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/data", "mysql_3306"), path)
}

func TestSafeJoin_NestedPath(t *testing.T) {
	path, err := SafeJoin("/data", "mysql_3306/my.cnf")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/data", "mysql_3306", "my.cnf"), path)
}

func TestSafeJoin_TraversalAttack(t *testing.T) {
	_, err := SafeJoin("/data", "../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestSafeJoin_DoubleDotNested(t *testing.T) {
	_, err := SafeJoin("/data", "mysql_3306/../../../etc/shadow")
	assert.Error(t, err)
}

func TestSafeJoin_SameDir(t *testing.T) {
	path, err := SafeJoin("/data", ".")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Clean("/data"), path)
}
