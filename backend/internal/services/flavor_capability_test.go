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
		for _, capability := range allCapabilities() {
			assert.True(t, HasCapability(flavor, capability),
				"flavor %q should allow %s", flavor, capability)
		}
	}
}

func TestHasCapabilityForMySQLCompatibleFlavors(t *testing.T) {
	for _, flavor := range []string{"mysql", "mariadb", "percona", "oceanbase", "gaussdb-mysql", "polardb-mysql", "tdsql-mysql"} {
		for _, capability := range allCapabilities() {
			assert.True(t, HasCapability(flavor, capability), "%s should allow %s", flavor, capability)
		}
	}
}

func TestEveryCapabilityIsLabelledAndRegistered(t *testing.T) {
	// A capability without a label produces an opaque error message, and a
	// capability missing from a flavor's map silently reads as false.
	for _, capability := range allCapabilities() {
		assert.NotEmpty(t, capabilityLabels[capability], "capability %s needs a label", capability)
	}
	for flavor, caps := range flavorCapabilities {
		for _, capability := range allCapabilities() {
			_, ok := caps[capability]
			assert.True(t, ok, "flavor %s is missing an explicit entry for %s", flavor, capability)
		}
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
			// Every MySQL-specific operation must be refused regardless of whether
			// the engine has a usable driver.
			for _, capability := range allCapabilities() {
				if capability == CapSQLHealthCheck {
					continue
				}
				assert.False(t, HasCapability(tt.flavor, capability),
					"%s must not allow %s", tt.flavor, capability)
			}
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
