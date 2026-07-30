package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasCapabilityTreatsUnknownAndEmptyFlavorAsMySQL(t *testing.T) {
	// Instances registered before flavor persistence carry an empty flavor and
	// must keep their full capability set.
	for _, flavor := range []string{"", "   ", "some-future-engine"} {
		assert.True(t, HasCapability(flavor, CapFailover), "flavor %q should allow failover", flavor)
		assert.True(t, HasCapability(flavor, CapReplication), "flavor %q should allow replication", flavor)
		assert.True(t, HasCapability(flavor, CapClusterDeploy), "flavor %q should allow cluster deploy", flavor)
		assert.True(t, HasCapability(flavor, CapPhysicalBackup), "flavor %q should allow physical backup", flavor)
		assert.True(t, HasCapability(flavor, CapInPlaceUpgrade), "flavor %q should allow in-place upgrade", flavor)
		assert.True(t, HasCapability(flavor, CapSQLHealthCheck), "flavor %q should allow SQL health check", flavor)
	}
}

func TestHasCapabilityForMySQLCompatibleFlavors(t *testing.T) {
	for _, flavor := range []string{"mysql", "mariadb", "percona", "oceanbase", "gaussdb-mysql", "polardb-mysql", "tdsql-mysql"} {
		assert.True(t, HasCapability(flavor, CapFailover), "%s should allow failover", flavor)
		assert.True(t, HasCapability(flavor, CapClusterDeploy), "%s should allow cluster deploy", flavor)
		assert.True(t, HasCapability(flavor, CapSQLHealthCheck), "%s should allow SQL health check", flavor)
	}
}

func TestHasCapabilityForTieredOnboardingFlavors(t *testing.T) {
	tests := []struct {
		flavor           string
		wantSQLHealtheck bool
	}{
		{flavor: "kingbase", wantSQLHealtheck: true},
		{flavor: "opengauss", wantSQLHealtheck: true},
		{flavor: "highgo", wantSQLHealtheck: true},
		{flavor: "gbase8a", wantSQLHealtheck: true},
		{flavor: "shentong", wantSQLHealtheck: true},
		{flavor: "dm", wantSQLHealtheck: false},
		{flavor: "gbase8s", wantSQLHealtheck: false},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			assert.False(t, HasCapability(tt.flavor, CapFailover), "%s must not allow failover", tt.flavor)
			assert.False(t, HasCapability(tt.flavor, CapReplication), "%s must not allow replication", tt.flavor)
			assert.False(t, HasCapability(tt.flavor, CapClusterDeploy), "%s must not allow cluster deploy", tt.flavor)
			assert.False(t, HasCapability(tt.flavor, CapPhysicalBackup), "%s must not allow physical backup", tt.flavor)
			assert.False(t, HasCapability(tt.flavor, CapInPlaceUpgrade), "%s must not allow in-place upgrade", tt.flavor)
			assert.Equal(t, tt.wantSQLHealtheck, HasCapability(tt.flavor, CapSQLHealthCheck))
		})
	}
}

func TestHasCapabilityNormalizesFlavorCase(t *testing.T) {
	assert.False(t, HasCapability("  GBase8S  ", CapSQLHealthCheck))
	assert.True(t, HasCapability("  KingBase  ", CapSQLHealthCheck))
	assert.False(t, HasCapability("DM", CapFailover))
}

func TestRequireCapabilityReturnsNilWhenSupported(t *testing.T) {
	require.NoError(t, RequireCapability("mysql", CapFailover))
	require.NoError(t, RequireCapability("", CapClusterDeploy))
	require.NoError(t, RequireCapability("kingbase", CapSQLHealthCheck))
}

func TestRequireCapabilityErrorNamesFlavorAndCapability(t *testing.T) {
	err := RequireCapability("gbase8s", CapFailover)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gbase8s")
	assert.Contains(t, err.Error(), string(CapFailover))
	assert.Contains(t, err.Error(), "故障切换")

	err = RequireCapability("dm", CapPhysicalBackup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dm")
	assert.Contains(t, err.Error(), string(CapPhysicalBackup))
}
