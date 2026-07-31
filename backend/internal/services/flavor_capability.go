package services

import (
	"context"
	"fmt"
	"strings"
)

// Capability names a platform operation that is not universally supported
// across every database engine the platform can manage.
//
// The platform's operational logic (failover, replication, physical backup,
// in-place upgrade, MySQL-architecture deployment) is written against the
// MySQL protocol and MySQL-specific SQL. Engines that only speak another
// protocol must be refused at the service entry point with a readable error
// rather than silently executing MySQL statements against them.
type Capability string

const (
	// CapReplication covers CHANGE MASTER / START SLAVE style replication setup.
	CapReplication Capability = "replication"
	// CapFailover covers automatic and manual master failover.
	CapFailover Capability = "failover"
	// CapClusterDeploy covers MGR / PXC / MHA / HA architecture deployment.
	CapClusterDeploy Capability = "cluster_deploy"
	// CapPhysicalBackup covers xtrabackup-based physical backup and restore.
	CapPhysicalBackup Capability = "backup_physical"
	// CapInPlaceUpgrade covers mysql_upgrade based in-place version upgrade.
	CapInPlaceUpgrade Capability = "upgrade_inplace"
	// CapSQLHealthCheck covers establishing a SQL connection for health checks.
	CapSQLHealthCheck Capability = "health_sql"
	// CapScale covers adding and removing cluster nodes, which drives the MySQL
	// kernel and architecture plugins.
	CapScale Capability = "scale"
	// CapNodeRebuild covers tearing down and reprovisioning a node's data via the
	// MySQL kernel plugin.
	CapNodeRebuild Capability = "node_rebuild"
	// CapInstanceDeploy covers deploying a single MySQL instance onto a host.
	CapInstanceDeploy Capability = "instance_deploy"
	// CapLogicalUpgrade covers mysqldump/mysqlpump based logical upgrade migration.
	CapLogicalUpgrade Capability = "upgrade_logical"
	// CapParameterTemplate covers pushing my.cnf parameter templates to an instance.
	CapParameterTemplate Capability = "parameter_template"
	// CapInstanceAdmin covers the agent instance-admin task family: MySQL account
	// management, SET GLOBAL, my.cnf read/write, service control and decommission.
	CapInstanceAdmin Capability = "instance_admin"
)

// capabilityLabels provides human-readable Chinese names used in error messages.
var capabilityLabels = map[Capability]string{
	CapReplication:       "主从复制搭建",
	CapFailover:          "故障切换",
	CapClusterDeploy:     "集群架构部署",
	CapPhysicalBackup:    "物理备份/恢复 (xtrabackup)",
	CapInPlaceUpgrade:    "原地版本升级",
	CapSQLHealthCheck:    "SQL 连接健康检测",
	CapScale:             "集群节点扩缩容",
	CapNodeRebuild:       "节点重建",
	CapInstanceDeploy:    "单实例部署",
	CapLogicalUpgrade:    "逻辑迁移升级",
	CapParameterTemplate: "参数模板下发",
	CapInstanceAdmin:     "实例管理操作 (账号/参数/配置/服务控制)",
}

// mysqlProtocolCapabilities is the capability set granted to engines that speak
// the MySQL wire protocol and accept MySQL-compatible administrative SQL.
func mysqlProtocolCapabilities() map[Capability]bool {
	return map[Capability]bool{
		CapReplication:       true,
		CapFailover:          true,
		CapClusterDeploy:     true,
		CapPhysicalBackup:    true,
		CapInPlaceUpgrade:    true,
		CapSQLHealthCheck:    true,
		CapScale:             true,
		CapNodeRebuild:       true,
		CapInstanceDeploy:    true,
		CapLogicalUpgrade:    true,
		CapParameterTemplate: true,
		CapInstanceAdmin:     true,
	}
}

// tieredOnboardingCapabilities is the capability set for engines the platform
// only onboards for inventory, health and topology display. No MySQL-specific
// operation is permitted. sqlHealthCheck is separate because some engines have
// no usable Go driver at all and can only be probed over TCP.
func tieredOnboardingCapabilities(sqlHealthCheck bool) map[Capability]bool {
	return map[Capability]bool{
		CapReplication:       false,
		CapFailover:          false,
		CapClusterDeploy:     false,
		CapPhysicalBackup:    false,
		CapInPlaceUpgrade:    false,
		CapSQLHealthCheck:    sqlHealthCheck,
		CapScale:             false,
		CapNodeRebuild:       false,
		CapInstanceDeploy:    false,
		CapLogicalUpgrade:    false,
		CapParameterTemplate: false,
		CapInstanceAdmin:     false,
	}
}

// flavorCapabilities is the single source of truth for what the platform will
// do with each engine flavor. Adding a new flavor means adding one entry here.
//
// Flavors absent from this map are treated as MySQL-compatible. That default is
// deliberate: existing managed instances predate flavor persistence and carry
// an empty flavor, and they must keep working exactly as before.
var flavorCapabilities = map[string]map[Capability]bool{
	// ---- MySQL wire protocol, MySQL-compatible admin SQL ----
	"mysql":         mysqlProtocolCapabilities(),
	"mariadb":       mysqlProtocolCapabilities(),
	"percona":       mysqlProtocolCapabilities(),
	"oceanbase":     mysqlProtocolCapabilities(),
	"gaussdb-mysql": mysqlProtocolCapabilities(),
	"polardb-mysql": mysqlProtocolCapabilities(),
	"tdsql-mysql":   mysqlProtocolCapabilities(),

	// TiDB speaks the MySQL wire protocol, so it can be connected and queried
	// for health. Its replication, backup and lifecycle workflows differ from
	// MySQL and must remain unavailable until they have dedicated support.
	"tidb": tieredOnboardingCapabilities(true),

	// ---- PostgreSQL-compatible: onboarding only, SQL health check available ----
	"kingbase":  tieredOnboardingCapabilities(true),
	"opengauss": tieredOnboardingCapabilities(true),
	"highgo":    tieredOnboardingCapabilities(true),
	"gbase8a":   tieredOnboardingCapabilities(true),
	"shentong":  tieredOnboardingCapabilities(true),

	// ---- No usable Go driver: TCP and SSH process discovery only ----
	// dm speaks a proprietary protocol; its driver lives behind the dm_driver
	// build tag and is not compiled by default.
	"dm": tieredOnboardingCapabilities(false),
	// gbase8s speaks the Informix protocol; no pure-Go driver exists.
	"gbase8s": tieredOnboardingCapabilities(false),
}

// normalizeFlavor lowercases and trims a flavor, mapping the empty value to
// "mysql" so that instances registered before flavor persistence keep their
// full capability set.
func normalizeFlavor(flavor string) string {
	f := strings.ToLower(strings.TrimSpace(flavor))
	if f == "" {
		return "mysql"
	}
	return f
}

// HasCapability reports whether the given flavor supports an operation.
// Unknown flavors are treated as MySQL-compatible so that a newly detected
// engine never silently loses access to operations that used to work.
func HasCapability(flavor string, capability Capability) bool {
	caps, ok := flavorCapabilities[normalizeFlavor(flavor)]
	if !ok {
		caps = mysqlProtocolCapabilities()
	}
	return caps[capability]
}

// RequireCapability returns nil when the flavor supports the operation, or a
// readable error naming both the flavor and the unsupported operation.
func RequireCapability(flavor string, capability Capability) error {
	if HasCapability(flavor, capability) {
		return nil
	}
	label := capabilityLabels[capability]
	if label == "" {
		label = string(capability)
	}
	return fmt.Errorf("数据库类型 %s 不支持%s (capability %s); 该类型当前仅支持纳管、健康检测与拓扑展示",
		normalizeFlavor(flavor), label, capability)
}

// allCapabilities lists every capability the platform gates on. It exists so that
// tests can assert the matrix is complete for every registered flavor.
func allCapabilities() []Capability {
	return []Capability{
		CapReplication,
		CapFailover,
		CapClusterDeploy,
		CapPhysicalBackup,
		CapInPlaceUpgrade,
		CapSQLHealthCheck,
		CapScale,
		CapNodeRebuild,
		CapInstanceDeploy,
		CapLogicalUpgrade,
		CapParameterTemplate,
		CapInstanceAdmin,
	}
}

// resolveClusterFlavor returns the engine flavor of a cluster by hydrating its
// instances. ListByClusterID does not populate the version record, so each
// instance is re-read through GetByID.
//
// An empty result means "unknown", which HasCapability treats as
// MySQL-compatible, so pre-existing clusters are never blocked.
func resolveClusterFlavor(ctx context.Context, repo InstanceRepositoryInterface, clusterID string) string {
	if repo == nil || strings.TrimSpace(clusterID) == "" {
		return ""
	}
	items, err := repo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return ""
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(item.Version.Flavor)); v != "" {
			return v
		}
		full, err := repo.GetByID(ctx, item.ID)
		if err != nil || full == nil {
			continue
		}
		if v := strings.ToLower(strings.TrimSpace(full.Version.Flavor)); v != "" {
			return v
		}
	}
	return ""
}

// resolveInstanceFlavor returns the engine flavor of a single instance, or an
// empty string when it cannot be determined.
func resolveInstanceFlavor(ctx context.Context, repo InstanceRepositoryInterface, instanceID string) string {
	if repo == nil || strings.TrimSpace(instanceID) == "" {
		return ""
	}
	inst, err := repo.GetByID(ctx, instanceID)
	if err != nil || inst == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(inst.Version.Flavor))
}
