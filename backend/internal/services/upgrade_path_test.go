package services

import (
	"strings"
	"testing"
)

// TestIsValidUpgradePath_MySQL tests MySQL upgrades against the catalog + fallback rules.
func TestIsValidUpgradePath_MySQL(t *testing.T) {
	tests := []struct {
		name          string
		srcFlavor     string
		srcVer        string
		dstFlavor     string
		dstVer        string
		wantAllowed   bool
		wantReasonMsg string // substring to match when not allowed
	}{
		{
			name:        "same major.minor patch upgrade (5.7.43 → 5.7.44)",
			srcFlavor:   "mysql", srcVer: "5.7.43",
			dstFlavor: "mysql", dstVer: "5.7.44",
			wantAllowed: true,
		},
		{
			name:        "5.7 to 8.0 via catalog (5.7.44 → 8.0.36)",
			srcFlavor:   "mysql", srcVer: "5.7.44",
			dstFlavor: "mysql", dstVer: "8.0.36",
			wantAllowed: true,
		},
		{
			name:        "5.7 to 8.0 via catalog (5.7.44 → 8.0.35)",
			srcFlavor:   "mysql", srcVer: "5.7.44",
			dstFlavor: "mysql", dstVer: "8.0.35",
			wantAllowed: true,
		},
		{
			name:        "5.6 to 8.0 blocked by catalog (5.6.51 → 8.0.36)",
			srcFlavor:   "mysql", srcVer: "5.6.51",
			dstFlavor: "mysql", dstVer: "8.0.36",
			wantAllowed:   false,
			wantReasonMsg: "does not list 5.6.51",
		},
		{
			name:        "5.7 to 5.6 downgrade blocked by catalog",
			srcFlavor:   "mysql", srcVer: "5.7.44",
			dstFlavor: "mysql", dstVer: "5.6.51",
			wantAllowed:   false,
			wantReasonMsg: "does not list 5.7.44",
		},
		{
			name:        "8.0 patch upgrade allowed",
			srcFlavor:   "mysql", srcVer: "8.0.34",
			dstFlavor: "mysql", dstVer: "8.0.36",
			wantAllowed: true,
		},
		{
			name:        "8.0 to 8.4 via catalog",
			srcFlavor:   "mysql", srcVer: "8.0.36",
			dstFlavor: "mysql", dstVer: "8.4.0",
			wantAllowed: true,
		},
		{
			name:        "default empty flavor is mysql (5.7 → 8.0)",
			srcFlavor:   "", srcVer: "5.7.44",
			dstFlavor: "", dstVer: "8.0.36",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsValidUpgradePath(tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer)
			if allowed != tt.wantAllowed {
				t.Errorf("IsValidUpgradePath(%q,%q → %q,%q) = (%v, %q), want allowed=%v",
					tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer,
					allowed, reason, tt.wantAllowed)
			}
			if !allowed && tt.wantReasonMsg != "" && !strings.Contains(reason, tt.wantReasonMsg) {
				t.Errorf("reason %q does not contain expected substring %q", reason, tt.wantReasonMsg)
			}
			if allowed && reason != "" {
				t.Errorf("allowed path should have empty reason, got %q", reason)
			}
		})
	}
}

// TestIsValidUpgradePath_FallbackRules tests upgrade paths where the target version
// is NOT in the catalog, exercising the fallback rules directly.
func TestIsValidUpgradePath_FallbackRules(t *testing.T) {
	tests := []struct {
		name          string
		srcFlavor     string
		srcVer        string
		dstFlavor     string
		dstVer        string
		wantAllowed   bool
		wantReasonMsg string
	}{
		// MySQL fallback (version not in catalog)
		{
			name:        "MySQL fallback 5.7 → 8.0 (version not in catalog)",
			srcFlavor:   "mysql", srcVer: "5.7.99",
			dstFlavor: "mysql", dstVer: "8.0.99",
			wantAllowed: true,
		},
		{
			name:        "MySQL fallback 5.6 → 8.0 blocked (go through 5.7)",
			srcFlavor:   "mysql", srcVer: "5.6.99",
			dstFlavor: "mysql", dstVer: "8.0.99",
			wantAllowed:   false,
			wantReasonMsg: "go through 5.7 first",
		},
		{
			name:        "MySQL fallback downgrade 8.0 → 5.7 blocked",
			srcFlavor:   "mysql", srcVer: "8.0.99",
			dstFlavor: "mysql", dstVer: "5.7.99",
			wantAllowed:   false,
			wantReasonMsg: "downgrade",
		},
		// MariaDB fallback (using fake versions not in catalog)
		{
			name:        "MariaDB fallback 10.5 → 10.6 (1 minor jump) allowed",
			srcFlavor:   "mariadb", srcVer: "10.5.99",
			dstFlavor: "mariadb", dstVer: "10.6.99",
			wantAllowed: true,
		},
		{
			name:        "MariaDB fallback 10.6 → 10.11 (5 minor jump) blocked",
			srcFlavor:   "mariadb", srcVer: "10.6.99",
			dstFlavor: "mariadb", dstVer: "10.11.99",
			wantAllowed:   false,
			wantReasonMsg: "spans 5 minor versions",
		},
		{
			name:        "MariaDB fallback 10.8 → 10.11 (3 minor jump) allowed",
			srcFlavor:   "mariadb", srcVer: "10.8.99",
			dstFlavor: "mariadb", dstVer: "10.11.99",
			wantAllowed: true,
		},
		{
			name:        "MariaDB fallback downgrade 10.11 → 10.6 blocked",
			srcFlavor:   "mariadb", srcVer: "10.11.99",
			dstFlavor: "mariadb", dstVer: "10.6.99",
			wantAllowed:   false,
			wantReasonMsg: "downgrade",
		},
		// Percona fallback (version not in catalog → treated as MySQL)
		{
			name:        "Percona fallback 5.7 → 8.0 (via MySQL rules)",
			srcFlavor:   "percona", srcVer: "5.7.99",
			dstFlavor: "percona", dstVer: "8.0.99",
			wantAllowed: true,
		},
		{
			name:        "Percona fallback 5.6 → 8.0 blocked (same as MySQL)",
			srcFlavor:   "percona", srcVer: "5.6.99",
			dstFlavor: "percona", dstVer: "8.0.99",
			wantAllowed:   false,
			wantReasonMsg: "go through 5.7 first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsValidUpgradePath(tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer)
			if allowed != tt.wantAllowed {
				t.Errorf("IsValidUpgradePath(%q,%q → %q,%q) = (%v, %q), want allowed=%v",
					tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer,
					allowed, reason, tt.wantAllowed)
			}
			if !allowed && tt.wantReasonMsg != "" && !strings.Contains(reason, tt.wantReasonMsg) {
				t.Errorf("reason %q does not contain expected substring %q", reason, tt.wantReasonMsg)
			}
			if allowed && reason != "" {
				t.Errorf("allowed path should have empty reason, got %q", reason)
			}
		})
	}
}

// TestIsValidUpgradePath_CrossFlavor tests that cross-flavor upgrades are always rejected.
func TestIsValidUpgradePath_CrossFlavor(t *testing.T) {
	tests := []struct {
		name          string
		srcFlavor     string
		srcVer        string
		dstFlavor     string
		dstVer        string
		wantAllowed   bool
		wantReasonMsg string
	}{
		{
			name:        "MySQL → MariaDB",
			srcFlavor:   "mysql", srcVer: "8.0.36",
			dstFlavor: "mariadb", dstVer: "10.11.4",
			wantAllowed:   false,
			wantReasonMsg: "cross-flavor",
		},
		{
			name:        "MariaDB → MySQL",
			srcFlavor:   "mariadb", srcVer: "10.11.4",
			dstFlavor: "mysql", dstVer: "8.0.36",
			wantAllowed:   false,
			wantReasonMsg: "cross-flavor",
		},
		{
			name:        "Percona → MySQL",
			srcFlavor:   "percona", srcVer: "8.0.36-28",
			dstFlavor: "mysql", dstVer: "8.0.36",
			wantAllowed:   false,
			wantReasonMsg: "cross-flavor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsValidUpgradePath(tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer)
			if allowed != tt.wantAllowed {
				t.Errorf("IsValidUpgradePath(%q,%q → %q,%q) = (%v, %q), want allowed=%v",
					tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer,
					allowed, reason, tt.wantAllowed)
			}
			if !allowed && tt.wantReasonMsg != "" && !strings.Contains(reason, tt.wantReasonMsg) {
				t.Errorf("reason %q does not contain expected substring %q", reason, tt.wantReasonMsg)
			}
		})
	}
}

// TestIsValidUpgradePath_PerconaCatalogEntry tests Percona with a catalog entry.
// When Percona has a catalog entry with UpgradeFrom, the catalog is authoritative.
func TestIsValidUpgradePath_PerconaCatalogEntry(t *testing.T) {
	tests := []struct {
		name          string
		srcFlavor     string
		srcVer        string
		dstFlavor     string
		dstVer        string
		wantAllowed   bool
		wantReasonMsg string
	}{
		{
			name:        "Percona catalog: same major.minor patch (8.0.35-27 → 8.0.36-28)",
			srcFlavor:   "percona", srcVer: "8.0.35-27",
			dstFlavor: "percona", dstVer: "8.0.36-28",
			wantAllowed: true,
		},
		{
			name:        "Percona catalog: 5.7 → 8.0 blocked (UpgradeFrom only lists 8.0.x)",
			srcFlavor:   "percona", srcVer: "5.7.44",
			dstFlavor: "percona", dstVer: "8.0.36-28",
			wantAllowed:   false,
			wantReasonMsg: "does not list 5.7.44",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsValidUpgradePath(tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer)
			if allowed != tt.wantAllowed {
				t.Errorf("IsValidUpgradePath(%q,%q → %q,%q) = (%v, %q), want allowed=%v",
					tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer,
					allowed, reason, tt.wantAllowed)
			}
			if !allowed && tt.wantReasonMsg != "" && !strings.Contains(reason, tt.wantReasonMsg) {
				t.Errorf("reason %q does not contain expected substring %q", reason, tt.wantReasonMsg)
			}
			if allowed && reason != "" {
				t.Errorf("allowed path should have empty reason, got %q", reason)
			}
		})
	}
}

// TestIsValidUpgradePath_MariaDB tests MariaDB with catalog entries.
// Catalog entries are authoritative when present.
func TestIsValidUpgradePath_MariaDB(t *testing.T) {
	tests := []struct {
		name          string
		srcFlavor     string
		srcVer        string
		dstFlavor     string
		dstVer        string
		wantAllowed   bool
		wantReasonMsg string
	}{
		{
			name:        "MariaDB catalog: same minor patch (10.11.3 → 10.11.4)",
			srcFlavor:   "mariadb", srcVer: "10.11.3",
			dstFlavor: "mariadb", dstVer: "10.11.4",
			wantAllowed: true,
		},
		{
			name:        "MariaDB catalog: 10.6.15 → 10.6.16 patch",
			srcFlavor:   "mariadb", srcVer: "10.6.15",
			dstFlavor: "mariadb", dstVer: "10.6.16",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsValidUpgradePath(tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer)
			if allowed != tt.wantAllowed {
				t.Errorf("IsValidUpgradePath(%q,%q → %q,%q) = (%v, %q), want allowed=%v",
					tt.srcFlavor, tt.srcVer, tt.dstFlavor, tt.dstVer,
					allowed, reason, tt.wantAllowed)
			}
			if allowed && reason != "" {
				t.Errorf("allowed path should have empty reason, got %q", reason)
			}
		})
	}
}

func TestMinorPart(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"10.11", 11},
		{"10.6", 6},
		{"10.0", 0},
		{"8.0", 0},
		{"5.7", 7},
		{"5", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := minorPart(tt.input)
			if got != tt.want {
				t.Errorf("minorPart(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionLessThan(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"5.7", "8.0", true},
		{"8.0", "8.0", false},
		{"8.0", "5.7", false},
		{"10.6", "10.11", true},
		{"10.11", "10.6", false},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := versionLessThan(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("versionLessThan(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"8.0.36", "8.0"},
		{"5.7.44", "5.7"},
		{"10.11.4", "10.11"},
		{"8.0", "8.0"},
		{"5", "5"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MajorVersion(tt.input)
			if got != tt.want {
				t.Errorf("MajorVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
