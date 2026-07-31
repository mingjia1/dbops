package services

import "strings"

// PackageDelivery describes how a database package reaches the target host.
// Open-source engines use vendor-published artifacts while commercial engines
// require an authorized, vendor-delivered package.
type PackageDelivery string

const (
	PackageDeliveryOfficial PackageDelivery = "official"
	PackageDeliveryLicensed PackageDelivery = "licensed"
)

// FlavorRelease is the version baseline used to prepare a flavor's local
// installation media. Commercial releases require an authorized package before
// their exact build, release date, and checksum can be accepted for execution.
type FlavorRelease struct {
	Flavor                     string
	Version                    string
	SourceURL                  string
	Delivery                   PackageDelivery
	ReleaseDate                string
	QueriedAt                  string
	RequiresVendorConfirmation bool
}

var flavorReleaseCatalog = map[string]FlavorRelease{
	"oceanbase": {
		Flavor: "oceanbase", Version: "v4.4.2_CE_BP2", ReleaseDate: "2026-07-21", QueriedAt: "2026-07-31",
		SourceURL: "https://github.com/oceanbase/oceanbase/releases/tag/v4.4.2_CE_BP2", Delivery: PackageDeliveryOfficial,
	},
	"gaussdb-mysql": vendorRelease("gaussdb-mysql", "MySQL 8.0", "https://www.huaweicloud.com/intl/en-us/product/gaussdb.html"),
	"polardb-mysql": vendorRelease("polardb-mysql", "MySQL 8.0", "https://www.alibabacloud.com/help/polardb/latest/polardb-for-mysql"),
	"tdsql-mysql":   vendorRelease("tdsql-mysql", "MySQL/MariaDB", "https://www.tencentcloud.com/products/tdsql"),
	"tidb": {
		Flavor: "tidb", Version: "v8.5.7", ReleaseDate: "2026-07-09", QueriedAt: "2026-07-31",
		SourceURL: "https://github.com/pingcap/tidb/releases/tag/v8.5.7", Delivery: PackageDeliveryOfficial,
	},
	"kingbase": vendorRelease("kingbase", "KES V9R1C10", "https://docs.kingbase.com.cn/cn/KES-V9R1C10/introduction"),
	"opengauss": {
		Flavor: "opengauss", Version: "6.0.5", ReleaseDate: "2026-05-15", QueriedAt: "2026-07-31",
		SourceURL: "https://opengauss.org/en/download/?version=all", Delivery: PackageDeliveryOfficial,
	},
	"highgo":   vendorRelease("highgo", "HGDB V9.0", "https://www.highgo.com/"),
	"gbase8a":  vendorRelease("gbase8a", "vendor-supported", "https://www.gbase.cn/product/gbase-8a"),
	"shentong": vendorRelease("shentong", "vendor-supported", "http://shentongdata.com/index.php/download/list-27"),
	"dm":       vendorRelease("dm", "DM9", "https://www.dameng.com/download/index.html"),
	"gbase8s":  vendorRelease("gbase8s", "vendor-supported", "https://www.gbase.cn/product/gbase-8s"),
}

func vendorRelease(flavor, version, sourceURL string) FlavorRelease {
	return FlavorRelease{
		Flavor: flavor, Version: version, SourceURL: sourceURL, Delivery: PackageDeliveryLicensed,
		QueriedAt:                  "2026-07-31",
		RequiresVendorConfirmation: true,
	}
}

// ReleaseForFlavor returns a copy of the catalog entry for a managed flavor.
// The empty flavor follows the legacy MySQL path and has no 信创 package baseline.
func ReleaseForFlavor(flavor string) (FlavorRelease, bool) {
	release, ok := flavorReleaseCatalog[strings.ToLower(strings.TrimSpace(flavor))]
	return release, ok
}
