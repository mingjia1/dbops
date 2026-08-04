package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasCapabilityPreservesEmptyFlavorAndRestrictsUnknownFlavor(t *testing.T) {
	// Instances registered before flavor persistence carry an empty flavor and
	// must keep their full capability set.
	for _, flavor := range []string{"", "   "} {
		for _, capability := range allCapabilities() {
			assert.True(t, HasCapability(flavor, capability),
				"flavor %q should allow %s", flavor, capability)
		}
	}
	for _, flavor := range []string{"unknown", "non-mysql", "tcp-only", "some-future-engine"} {
		for _, capability := range allCapabilities() {
			assert.False(t, HasCapability(flavor, capability),
				"flavor %q should refuse %s", flavor, capability)
		}
	}
}

func TestHasCapabilityForMySQLLifecycleFlavors(t *testing.T) {
	for _, flavor := range []string{"mysql", "mariadb", "percona"} {
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
		{flavor: "tidb", wantSQLHealtheck: true},
		{flavor: "oceanbase", wantSQLHealtheck: true},
		{flavor: "gaussdb-mysql", wantSQLHealtheck: true},
		{flavor: "polardb-mysql", wantSQLHealtheck: true},
		{flavor: "tdsql-mysql", wantSQLHealtheck: true},
		{flavor: "dm", wantSQLHealtheck: false},
		{flavor: "gbase8s", wantSQLHealtheck: false},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			allowed := map[Capability]bool{}
			if tt.flavor == "oceanbase" {
				allowed[CapInstanceDeploy] = true
				allowed[CapParameterTemplate] = true
				allowed[CapPhysicalBackup] = true
				allowed[CapInPlaceUpgrade] = true
			}
			if tt.flavor == "tidb" {
				allowed[CapInstanceDeploy] = true
				allowed[CapParameterTemplate] = true
				allowed[CapPhysicalBackup] = true
				allowed[CapInPlaceUpgrade] = true
				allowed[CapInstanceAdmin] = true
			}
			// Only operations backed by the flavor executor are available.
			for _, capability := range allCapabilities() {
				if capability == CapSQLHealthCheck || allowed[capability] {
					continue
				}
				assert.False(t, HasCapability(tt.flavor, capability),
					"%s must not allow %s", tt.flavor, capability)
			}
			assert.Equal(t, tt.wantSQLHealtheck, HasCapability(tt.flavor, CapSQLHealthCheck))
			if tt.flavor == "oceanbase" {
				assert.True(t, HasCapability(tt.flavor, CapPhysicalBackup))
				assert.True(t, HasCapability(tt.flavor, CapInPlaceUpgrade))
			}
			if tt.flavor == "tidb" {
				assert.True(t, HasCapability(tt.flavor, CapInstanceDeploy))
				assert.True(t, HasCapability(tt.flavor, CapParameterTemplate))
				assert.True(t, HasCapability(tt.flavor, CapPhysicalBackup))
				assert.True(t, HasCapability(tt.flavor, CapInPlaceUpgrade))
				assert.True(t, HasCapability(tt.flavor, CapInstanceAdmin))
			}
		})
	}
}

func TestSingleNodeExecutorCapabilitiesOnlyEnableCompletedSingleNodeOperations(t *testing.T) {
	caps := singleNodeExecutorCapabilities(true,
		CapInstanceDeploy,
		CapParameterTemplate,
		CapPhysicalBackup,
		CapLogicalUpgrade,
		CapReplication,
		CapFailover,
		CapScale,
		CapNodeRebuild,
	)

	for _, capability := range []Capability{CapInstanceDeploy, CapParameterTemplate, CapPhysicalBackup, CapLogicalUpgrade, CapSQLHealthCheck} {
		assert.True(t, caps[capability], "single-node executor should enable %s", capability)
	}
	for _, capability := range []Capability{CapReplication, CapFailover, CapScale, CapNodeRebuild, CapClusterDeploy} {
		assert.False(t, caps[capability], "single-node executor must keep %s disabled", capability)
	}
}

func TestXinchuangFlavorsHaveExplicitCompletedCapabilityRecords(t *testing.T) {
	for _, flavor := range []string{
		"oceanbase", "gaussdb-mysql", "polardb-mysql", "tdsql-mysql", "tidb", "kingbase",
		"opengauss", "highgo", "gbase8a", "shentong", "dm", "gbase8s",
	} {
		_, ok := completedSingleNodeCapabilities[flavor]
		assert.True(t, ok, "%s needs a completed executor capability record", flavor)
		for _, capability := range []Capability{CapReplication, CapFailover, CapScale, CapNodeRebuild} {
			assert.False(t, HasCapability(flavor, capability), "%s must keep %s disabled", flavor, capability)
		}
	}
}

func TestKingbaseKeepsDistributedLifecycleCapabilitiesDisabled(t *testing.T) {
	for _, capability := range []Capability{CapReplication, CapFailover, CapScale, CapNodeRebuild, CapClusterDeploy} {
		assert.False(t, HasCapability("kingbase", capability), "kingbase must keep %s disabled", capability)
		err := RequireCapability("kingbase", capability)
		require.Error(t, err)
		assert.Contains(t, err.Error(), capabilityLabels[capability])
	}
}

func TestGBase8aKeepsGenericLifecycleCapabilitiesDisabled(t *testing.T) {
	for _, capability := range []Capability{CapInstanceDeploy, CapParameterTemplate, CapInstanceAdmin, CapPhysicalBackup, CapLogicalUpgrade, CapInPlaceUpgrade} {
		assert.False(t, HasCapability("gbase8a", capability), "gbase8a generic path must keep %s disabled", capability)
		err := RequireCapability("gbase8a", capability)
		require.Error(t, err)
		assert.Contains(t, err.Error(), capabilityLabels[capability])
	}
}

func TestHasCapabilityNormalizesFlavorCase(t *testing.T) {
	assert.False(t, HasCapability("  GBase8S  ", CapSQLHealthCheck))
	assert.True(t, HasCapability("  KingBase  ", CapSQLHealthCheck))
	assert.False(t, HasCapability(" TiDB ", CapFailover))
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

	err = RequireCapability("dm", CapInPlaceUpgrade)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dm")
	assert.Contains(t, err.Error(), string(CapInPlaceUpgrade))
}
