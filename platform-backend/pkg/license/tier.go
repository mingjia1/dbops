package license

type Tier string

const (
	TierCE Tier = "ce"
	TierEE Tier = "ee"
	TierUE Tier = "ue"
)

type Feature string

const (
	FeatureInstanceManage  Feature = "instance_manage"
	FeatureBasicMonitor    Feature = "basic_monitor"
	FeatureBasicBackup     Feature = "basic_backup"
	FeatureBasicDeploy     Feature = "basic_deploy"
	FeatureBasicUpgrade    Feature = "basic_upgrade"
	FeatureDataMasking     Feature = "data_masking"
	FeatureAuditChain      Feature = "audit_chain"
	FeatureKeyRotation     Feature = "key_rotation"
	FeatureLDAP            Feature = "ldap"
	FeatureSSO             Feature = "sso"
	FeatureNotifyChannel   Feature = "notify_channel"
	FeatureAlertUpgrade    Feature = "alert_upgrade"
	FeatureCapacityPredict Feature = "capacity_predict"
	FeatureAutoFailover    Feature = "auto_failover"
	FeatureChaosTest       Feature = "chaos_test"
	FeatureAIAssistant     Feature = "ai_assistant"
	FeatureText2SQL        Feature = "text2sql"
	FeatureAutoTuning      Feature = "auto_tuning"
	FeatureIndexAutoCreate Feature = "index_auto_create"
	FeatureSQLOptimize     Feature = "sql_optimize"
	FeatureRootCause       Feature = "root_cause"
	FeatureAnomalyDetect   Feature = "anomaly_detect"
)

var tierFeatures = map[Tier][]Feature{
	TierCE: {FeatureInstanceManage, FeatureBasicMonitor, FeatureBasicBackup, FeatureBasicDeploy, FeatureBasicUpgrade},
	TierEE: {FeatureInstanceManage, FeatureBasicMonitor, FeatureBasicBackup, FeatureBasicDeploy, FeatureBasicUpgrade, FeatureDataMasking, FeatureAuditChain, FeatureKeyRotation, FeatureLDAP, FeatureSSO, FeatureNotifyChannel, FeatureAlertUpgrade, FeatureCapacityPredict, FeatureAutoFailover, FeatureChaosTest},
	TierUE: {FeatureInstanceManage, FeatureBasicMonitor, FeatureBasicBackup, FeatureBasicDeploy, FeatureBasicUpgrade, FeatureDataMasking, FeatureAuditChain, FeatureKeyRotation, FeatureLDAP, FeatureSSO, FeatureNotifyChannel, FeatureAlertUpgrade, FeatureCapacityPredict, FeatureAutoFailover, FeatureChaosTest, FeatureAIAssistant, FeatureText2SQL, FeatureAutoTuning, FeatureIndexAutoCreate, FeatureSQLOptimize, FeatureRootCause, FeatureAnomalyDetect},
}

func FeaturesForTier(t Tier) []Feature {
	if f, ok := tierFeatures[t]; ok {
		return f
	}
	return tierFeatures[TierCE]
}

func HasFeature(t Tier, f Feature) bool {
	for _, feat := range FeaturesForTier(t) {
		if feat == f {
			return true
		}
	}
	return false
}
