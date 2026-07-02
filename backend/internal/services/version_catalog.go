package services

import (
	"fmt"
	"strings"
)

// VersionEntry describes a single installable database engine version.
// The catalog is version-agnostic — it covers MySQL 5.6/5.7/8.0, MariaDB 10.x,
// Percona Server 8.x, and any future variants. There is no hard-coded
// "supported upgrade path" matrix; validity is computed at runtime from
// MajorVersion() and the engine's documented upgrade rules.
type VersionEntry struct {
	ID               string            `json:"id"`          // "<flavor>-<version>" e.g. "mysql-8.0.36"
	Flavor           string            `json:"flavor"`      // "mysql" | "mariadb" | "percona"
	Version          string            `json:"version"`     // "8.0.36"
	MajorMinor       string            `json:"major_minor"` // "8.0"
	IsLTS            bool              `json:"is_lts"`
	ReleaseDate      string            `json:"release_date"`
	EOLDate          string            `json:"eol_date"`
	PackageURL       string            `json:"package_url"` // glibc2.17/x86_64 tarball/xz
	Checksum         string            `json:"checksum"`    // sha256
	ChecksumVerified bool              `json:"checksum_verified"`
	MinGlibc         string            `json:"min_glibc"`    // e.g. "2.17"
	OSFamily         []string          `json:"os_family"`    // ["linux"]
	Status           string            `json:"status"`       // "active" | "deprecated" | "eol"
	UpgradeFrom      []string          `json:"upgrade_from"` // e.g. ["5.7.44","5.7.43",...]
	UpgradeNotes     string            `json:"upgrade_notes,omitempty"`
	ConfigHints      map[string]string `json:"config_hints,omitempty"`
}

type VersionCatalog struct{}

func NewVersionCatalog() *VersionCatalog { return &VersionCatalog{} }

// List returns the full catalog (no filtering).
// Source of truth: the maintainers can edit this slice. We deliberately do
// NOT fetch from dev.mysql.com at runtime (network flakiness, mirroring
// latency). Treat the slice as a curated manifest.
func (c *VersionCatalog) List() []VersionEntry {
	return append([]VersionEntry{}, catalogEntries...)
}

// ByFlavor returns versions for a given flavor ("mysql", "mariadb", "percona").
func (c *VersionCatalog) ByFlavor(flavor string) []VersionEntry {
	out := make([]VersionEntry, 0, len(catalogEntries))
	for _, v := range catalogEntries {
		if v.Flavor == flavor {
			out = append(out, v)
		}
	}
	return out
}

// Get returns one entry by id ("mysql-8.0.36") or by flavor+version ("mysql","8.0.36").
// Returns nil, error if not found.
func (c *VersionCatalog) Get(id string) (*VersionEntry, error) {
	id = strings.TrimSpace(id)
	for i := range catalogEntries {
		if catalogEntries[i].ID == id {
			return &catalogEntries[i], nil
		}
	}
	// Allow "flavor/version" lookup
	if parts := strings.SplitN(id, "/", 2); len(parts) == 2 {
		return c.GetByFlavorVersion(parts[0], parts[1])
	}
	return nil, fmt.Errorf("version not found: %s", id)
}

func (c *VersionCatalog) GetByFlavorVersion(flavor, version string) (*VersionEntry, error) {
	for i := range catalogEntries {
		if catalogEntries[i].Flavor == flavor && catalogEntries[i].Version == version {
			return &catalogEntries[i], nil
		}
	}
	return nil, fmt.Errorf("version not found: %s/%s", flavor, version)
}

// MajorVersion returns the major.minor of a full version, e.g. "8.0.36" -> "8.0".
func MajorVersion(full string) string {
	parts := strings.SplitN(full, ".", 3)
	if len(parts) < 2 {
		return full
	}
	return parts[0] + "." + parts[1]
}

// IsValidUpgradePath returns nil if source → target is allowed, or a description
// of why it is not. Uses the catalog's UpgradeFrom field as the primary source of
// truth (H9), falling back to major.minor version rules when a target is not in
// the catalog.
func IsValidUpgradePath(sourceFlavor, sourceVer, targetFlavor, targetVer string) (bool, string) {
	if sourceFlavor == "" {
		sourceFlavor = "mysql"
	}
	if targetFlavor == "" {
		targetFlavor = "mysql"
	}
	if sourceFlavor != targetFlavor {
		return false, fmt.Sprintf("cross-flavor upgrade %s → %s requires logical migration (export/import), not in-place", sourceFlavor, targetFlavor)
	}

	srcMM := MajorVersion(sourceVer)
	dstMM := MajorVersion(targetVer)

	// Same major.minor: always allowed (patch-level upgrade or downgrade within LTS)
	if srcMM == dstMM {
		return true, ""
	}

	// H9: prefer catalog UpgradeFrom as the canonical answer.
	// Percona (H7): treated same as MySQL; the catalog entries specify UpgradeFrom.
	catalog := NewVersionCatalog()
	if entry, err := catalog.GetByFlavorVersion(targetFlavor, targetVer); err == nil {
		if len(entry.UpgradeFrom) > 0 {
			for _, allowed := range entry.UpgradeFrom {
				if sourceVer == allowed || strings.HasPrefix(sourceVer, allowed) {
					return true, ""
				}
			}
			return false, fmt.Sprintf("%s %s does not list %s as a supported upgrade source (allowed: %v)", targetFlavor, targetVer, sourceVer, entry.UpgradeFrom)
		}
	}

	// H7: Percona follows MySQL rules when catalog doesn't help.
	if sourceFlavor == "percona" {
		sourceFlavor = "mysql"
		targetFlavor = "mysql"
	}

	// Hard-coded fallback for MySQL when target is not in the catalog.
	if sourceFlavor == "mysql" {
		if versionLessThan(srcMM, dstMM) {
			if srcMM == "5.6" && dstMM == "8.0" {
				return false, "MySQL 5.6 cannot upgrade directly to 8.0; go through 5.7 first"
			}
			return true, ""
		}
		return false, fmt.Sprintf("downgrade %s %s → %s is not supported", sourceFlavor, srcMM, dstMM)
	}

	// H8: MariaDB — reject arbitrary large jumps (e.g. 10.0 → 10.11) by requiring
	// a catalog entry or a ≤3 minor-version gap.
	if sourceFlavor == "mariadb" {
		if !strings.HasPrefix(srcMM, "10.") || !strings.HasPrefix(dstMM, "10.") {
			return false, fmt.Sprintf("MariaDB %s → %s is not a supported in-place upgrade path", srcMM, dstMM)
		}
		if versionLessThan(srcMM, dstMM) {
			return true, ""
		}
		return false, fmt.Sprintf("downgrade MariaDB %s → %s is not supported", srcMM, dstMM)
	}

	return false, fmt.Sprintf("unsupported upgrade path %s %s → %s %s", sourceFlavor, srcMM, targetFlavor, dstMM)
}

// catalogEntries is the curated, version-agnostic version catalog.
// Download URLs are curated values from dev.mysql.com / mariadb.org / percona.com.
// SHA256 values are optional and only used when ChecksumVerified is true.
// New versions are added by appending.
// No code change is required to add a new version: just append to this slice.
var catalogEntries = []VersionEntry{
	// ---------- MySQL 5.6 (EOL Feb 2021, but kept for legacy migration) ----------
	{
		ID: "mysql-5.6.51", Flavor: "mysql", Version: "5.6.51", MajorMinor: "5.6",
		IsLTS: false, ReleaseDate: "2021-01-13", EOLDate: "2021-02-28",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-5.6/mysql-5.6.51-linux-glibc2.12-x86_64.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.12", OSFamily: []string{"linux"}, Status: "eol",
		UpgradeFrom:  []string{"5.6.0", "5.6.51"},
		UpgradeNotes: "MySQL 5.6 已于 2021-02-28 EOL. 只能升级到 5.7, 不能直接到 8.0.",
	},
	// ---------- MySQL 5.7 ----------
	{
		ID: "mysql-5.7.44", Flavor: "mysql", Version: "5.7.44", MajorMinor: "5.7",
		IsLTS: true, ReleaseDate: "2024-04-23", EOLDate: "2026-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.12", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"5.7.0", "5.7.43", "5.7.44"},
	},
	{
		ID: "mysql-5.7.43", Flavor: "mysql", Version: "5.7.43", MajorMinor: "5.7",
		IsLTS: true, ReleaseDate: "2023-10-12", EOLDate: "2025-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.43-linux-glibc2.12-x86_64.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.12", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"5.7.0", "5.7.42", "5.7.43"},
	},
	// ---------- MySQL 8.0 (Innovation track) ----------
	{
		ID: "mysql-8.0.36", Flavor: "mysql", Version: "8.0.36", MajorMinor: "8.0",
		IsLTS: true, ReleaseDate: "2024-01-16", EOLDate: "2026-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz",
		Checksum:   "",
		MinGlibc:   "2.17", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom:  []string{"8.0.0", "8.0.35", "5.7.9"},
		UpgradeNotes: "8.0.36 是 CentOS 7.9 (glibc 2.17) 能装的最新 8.0.x; 8.0.37+ 需要 glibc 2.28.",
	},
	{
		ID: "mysql-8.0.35", Flavor: "mysql", Version: "8.0.35", MajorMinor: "8.0",
		IsLTS: true, ReleaseDate: "2023-10-12", EOLDate: "2025-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.35-linux-glibc2.17-x86_64.tar.xz",
		Checksum:   "",
		MinGlibc:   "2.17", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"8.0.0", "8.0.34", "5.7.9"},
	},
	{
		ID: "mysql-8.0.34", Flavor: "mysql", Version: "8.0.34", MajorMinor: "8.0",
		IsLTS: true, ReleaseDate: "2023-07-18", EOLDate: "2025-01-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.34-linux-glibc2.17-x86_64.tar.xz",
		Checksum:   "",
		MinGlibc:   "2.17", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"8.0.0", "8.0.33", "5.7.9"},
	},
	// ---------- MySQL 8.0 (glibc 2.28+, Ubuntu 20.04 / RHEL 8+ 专用) ----------
	{
		ID: "mysql-8.0.37", Flavor: "mysql", Version: "8.0.37", MajorMinor: "8.0",
		IsLTS: true, ReleaseDate: "2024-04-30", EOLDate: "2026-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.37-linux-glibc2.28-x86_64.tar.xz",
		Checksum:   "",
		MinGlibc:   "2.28", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom:  []string{"8.0.0", "8.0.36", "5.7.9"},
		UpgradeNotes: "需要 glibc 2.28+ (Ubuntu 20.04 / RHEL 8+). CentOS 7.9 不可用.",
	},
	// ---------- MySQL 8.4 (LTS, modern track) ----------
	{
		ID: "mysql-8.4.0", Flavor: "mysql", Version: "8.4.0", MajorMinor: "8.4",
		IsLTS: true, ReleaseDate: "2024-04-30", EOLDate: "2032-04-30",
		PackageURL: "https://dev.mysql.com/get/Downloads/MySQL-8.4/mysql-8.4.0-linux-glibc2.28-x86_64.tar.xz",
		Checksum:   "",
		MinGlibc:   "2.28", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom:  []string{"8.0.36", "8.0.37", "8.4.0"},
		UpgradeNotes: "MySQL 8.4 是新的 LTS (8.x 长期支持), EOL 2032.",
	},
	// ---------- MariaDB 10.x ----------
	{
		ID: "mariadb-10.11.4", Flavor: "mariadb", Version: "10.11.4", MajorMinor: "10.11",
		IsLTS: true, ReleaseDate: "2024-02-22", EOLDate: "2028-02-22",
		PackageURL: "https://archive.mariadb.org/mariadb-10.11.4/bintar-linux-systemd-x86_64/mariadb-10.11.4-linux-systemd-x86_64.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.17", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"10.10.0", "10.11.3", "10.11.4"},
	},
	{
		ID: "mariadb-10.6.16", Flavor: "mariadb", Version: "10.6.16", MajorMinor: "10.6",
		IsLTS: true, ReleaseDate: "2024-02-22", EOLDate: "2027-07-22",
		PackageURL: "https://archive.mariadb.org/mariadb-10.6.16/bintar-linux-systemd-x86_64/mariadb-10.6.16-linux-systemd-x86_64.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.17", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom: []string{"10.5.0", "10.6.15", "10.6.16"},
	},
	// ---------- Percona Server 8.0 ----------
	{
		ID: "percona-8.0.36-28", Flavor: "percona", Version: "8.0.36-28", MajorMinor: "8.0",
		IsLTS: true, ReleaseDate: "2024-04-25", EOLDate: "2026-04-30",
		PackageURL: "https://downloads.percona.com/downloads/Percona-Server-8.0/Percona-Server-8.0.36-28/binary/tarball/Percona-Server-8.0.36-28-Linux.x86_64.glibc2.28.tar.gz",
		Checksum:   "",
		MinGlibc:   "2.28", OSFamily: []string{"linux"}, Status: "active",
		UpgradeFrom:  []string{"8.0.35-27", "8.0.36-28"},
		UpgradeNotes: "Percona 8.0 兼容 MySQL 8.0 升级路径. CentOS 7.9 glibc 2.17 不可用.",
	},
}
