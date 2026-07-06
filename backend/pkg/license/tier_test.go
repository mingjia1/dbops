package license

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasFeatureCE(t *testing.T) {
	assert.True(t, HasFeature(TierCE, FeatureInstanceManage))
	assert.True(t, HasFeature(TierCE, FeatureBasicMonitor))
	assert.True(t, HasFeature(TierCE, FeatureBasicBackup))
	assert.True(t, HasFeature(TierCE, FeatureBasicDeploy))
	assert.True(t, HasFeature(TierCE, FeatureBasicUpgrade))
	assert.False(t, HasFeature(TierCE, FeatureDataMasking))
	assert.False(t, HasFeature(TierCE, FeatureAIAssistant))
	assert.False(t, HasFeature(TierCE, FeatureAutoFailover))
}

func TestHasFeatureEE(t *testing.T) {
	assert.True(t, HasFeature(TierEE, FeatureInstanceManage))
	assert.True(t, HasFeature(TierEE, FeatureDataMasking))
	assert.True(t, HasFeature(TierEE, FeatureAuditChain))
	assert.True(t, HasFeature(TierEE, FeatureKeyRotation))
	assert.True(t, HasFeature(TierEE, FeatureLDAP))
	assert.True(t, HasFeature(TierEE, FeatureAutoFailover))
	assert.False(t, HasFeature(TierEE, FeatureAIAssistant))
	assert.False(t, HasFeature(TierEE, FeatureText2SQL))
}

func TestHasFeatureUE(t *testing.T) {
	for _, f := range FeaturesForTier(TierUE) {
		assert.True(t, HasFeature(TierUE, f), "UE should have feature %s", f)
	}
}

func TestFeaturesForTier(t *testing.T) {
	ceFeatures := FeaturesForTier(TierCE)
	eeFeatures := FeaturesForTier(TierEE)
	ueFeatures := FeaturesForTier(TierUE)

	assert.Len(t, ceFeatures, 5)
	assert.True(t, len(eeFeatures) > len(ceFeatures))
	assert.True(t, len(ueFeatures) > len(eeFeatures))
}

func TestLicenseDataSignAndVerify(t *testing.T) {
	secret := "test-secret-key"
	lic := &LicenseData{
		LicenseKey: "key-123",
		Tier:       TierEE,
		IssuedTo:   "test-user",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(365 * 24 * time.Hour),
		MaxNodes:   50,
	}

	lic.Sign(secret)
	assert.NotEmpty(t, lic.Signature)
	assert.True(t, lic.Verify(secret))
	assert.False(t, lic.Verify("wrong-secret"))
}

func TestLicenseDataIsExpired(t *testing.T) {
	lic := &LicenseData{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	assert.True(t, lic.IsExpired())

	lic.ExpiresAt = time.Now().Add(time.Hour)
	assert.False(t, lic.IsExpired())
}

func TestLicenseDataIsValid(t *testing.T) {
	secret := "test-secret"
	lic := &LicenseData{
		LicenseKey: "key",
		Tier:       TierCE,
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	lic.Sign(secret)
	assert.True(t, lic.IsValid())

	// Expired
	lic2 := &LicenseData{
		LicenseKey: "key2",
		Tier:       TierCE,
		ExpiresAt:  time.Now().Add(-time.Hour),
	}
	lic2.Sign(secret)
	assert.False(t, lic2.IsValid())
}

func TestLicenseDataFeatures(t *testing.T) {
	lic := &LicenseData{Tier: TierCE}
	features := lic.Features()
	assert.Len(t, features, 5)
}

func TestLicenseDataHasFeature(t *testing.T) {
	lic := &LicenseData{Tier: TierEE}
	assert.True(t, lic.HasFeature(FeatureDataMasking))
	assert.False(t, lic.HasFeature(FeatureAIAssistant))
}

func TestGenerateAndParse(t *testing.T) {
	secret := "test-secret"
	lic := Generate(TierEE, "test-org", 365*24*time.Hour, 50, secret)
	require.NotNil(t, lic)
	assert.Equal(t, TierEE, lic.Tier)
	assert.Equal(t, "test-org", lic.IssuedTo)
	assert.Equal(t, 50, lic.MaxNodes)
	assert.NotEmpty(t, lic.Signature)

	jsonData, err := json.Marshal(lic)
	require.NoError(t, err)
	parsed, err := Parse(jsonData, secret)
	require.NoError(t, err)
	assert.Equal(t, lic.Tier, parsed.Tier)
	assert.Equal(t, lic.IssuedTo, parsed.IssuedTo)
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte("not json"), "secret")
	require.Error(t, err)
}

func TestParseInvalidSignature(t *testing.T) {
	lic := Generate(TierCE, "user", time.Hour, 10, "secret1")
	jsonData, err := json.Marshal(lic)
	require.NoError(t, err)

	_, err = Parse(jsonData, "secret2")
	require.Error(t, err)
}
