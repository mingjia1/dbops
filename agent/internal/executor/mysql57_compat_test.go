package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- mysqlAuthPlugin ---

func TestMysqlAuthPlugin_MySQL57(t *testing.T) {
	assert.Equal(t, "mysql_native_password", mysqlAuthPlugin("5.7"))
	assert.Equal(t, "mysql_native_password", mysqlAuthPlugin("5.7.44"))
	assert.Equal(t, "mysql_native_password", mysqlAuthPlugin("5.6"))
}

func TestMysqlAuthPlugin_MySQL80(t *testing.T) {
	assert.Equal(t, "caching_sha2_password", mysqlAuthPlugin("8.0"))
	assert.Equal(t, "caching_sha2_password", mysqlAuthPlugin("8.0.36"))
	assert.Equal(t, "caching_sha2_password", mysqlAuthPlugin("8.4"))
	assert.Equal(t, "caching_sha2_password", mysqlAuthPlugin("8.4.3"))
}

func TestMysqlAuthPlugin_Empty(t *testing.T) {
	// Empty defaults to 8.0+ which uses caching_sha2_password
	assert.Equal(t, "caching_sha2_password", mysqlAuthPlugin(""))
}

// --- mysqlHasGetServerPublicKey ---

func TestMysqlHasGetServerPublicKey_MySQL57(t *testing.T) {
	assert.False(t, mysqlHasGetServerPublicKey("5.7"))
	assert.False(t, mysqlHasGetServerPublicKey("5.7.44"))
	assert.False(t, mysqlHasGetServerPublicKey("5.6"))
}

func TestMysqlHasGetServerPublicKey_MySQL80(t *testing.T) {
	assert.True(t, mysqlHasGetServerPublicKey("8.0"))
	assert.True(t, mysqlHasGetServerPublicKey("8.0.36"))
	assert.True(t, mysqlHasGetServerPublicKey("8.4"))
	assert.True(t, mysqlHasGetServerPublicKey("8.4.3"))
}

func TestMysqlHasGetServerPublicKey_Empty(t *testing.T) {
	// Empty defaults to 8.0+
	assert.True(t, mysqlHasGetServerPublicKey(""))
}

// --- MGRConfig: MySQLVersion parsing ---

func TestParseMGRConfig_MySQLVersion57(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{"mysql_version": "5.7"})
	assert.Equal(t, "5.7", got.MySQLVersion)
}

func TestParseMGRConfig_MySQLVersion80(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{"mysql_version": "8.0"})
	assert.Equal(t, "8.0", got.MySQLVersion)
}

func TestParseMGRConfig_MySQLVersion84(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{"mysql_version": "8.4"})
	assert.Equal(t, "8.4", got.MySQLVersion)
}

func TestParseMGRConfig_MySQLVersionEmpty(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{})
	assert.Empty(t, got.MySQLVersion)
}

func TestParseMGRConfig_MySQLVersionWrongType(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{"mysql_version": 5.7})
	assert.Empty(t, got.MySQLVersion)
}

// --- configString for mysql_version ---

func TestConfigString_MySQLVersion57(t *testing.T) {
	assert.Equal(t, "5.7", configString(map[string]interface{}{"mysql_version": "5.7"}, "mysql_version"))
}

func TestConfigString_MySQLVersion80(t *testing.T) {
	assert.Equal(t, "8.0", configString(map[string]interface{}{"mysql_version": "8.0"}, "mysql_version"))
}

func TestConfigString_MySQLVersionEmpty(t *testing.T) {
	assert.Equal(t, "", configString(map[string]interface{}{}, "mysql_version"))
}

// --- mysql_version default fallback ---

func TestMySQLVersionDefaultInDeployFlow(t *testing.T) {
	extractVersion := func(cfg map[string]interface{}) string {
		v := configString(cfg, "mysql_version")
		if v == "" {
			v = "8.0"
		}
		return v
	}

	assert.Equal(t, "8.0", extractVersion(map[string]interface{}{}))
	assert.Equal(t, "5.7", extractVersion(map[string]interface{}{"mysql_version": "5.7"}))
	assert.Equal(t, "8.0", extractVersion(map[string]interface{}{"mysql_version": "8.0"}))
	assert.Equal(t, "8.4", extractVersion(map[string]interface{}{"mysql_version": "8.4"}))
}

