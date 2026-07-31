package services

import "testing"

func TestReleaseForFlavorReturnsPublishedOpenSourceBaselines(t *testing.T) {
	tests := []struct {
		flavor  string
		version string
	}{
		{flavor: "oceanbase", version: "v4.4.2_CE_BP2"},
		{flavor: "tidb", version: "v8.5.7"},
		{flavor: "opengauss", version: "6.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.flavor, func(t *testing.T) {
			release, ok := ReleaseForFlavor(" " + tt.flavor + " ")
			if !ok {
				t.Fatal("release baseline not found")
			}
			if release.Version != tt.version {
				t.Fatalf("version = %q, want %q", release.Version, tt.version)
			}
			if release.Delivery != PackageDeliveryOfficial {
				t.Fatalf("delivery = %q, want %q", release.Delivery, PackageDeliveryOfficial)
			}
			if release.ReleaseDate == "" || release.SourceURL == "" || release.QueriedAt == "" {
				t.Fatal("published release must include its date, source, and query date")
			}
		})
	}
}

func TestReleaseForFlavorRequiresVendorConfirmationForLicensedPackages(t *testing.T) {
	release, ok := ReleaseForFlavor("dm")
	if !ok {
		t.Fatal("release baseline not found")
	}
	if release.Delivery != PackageDeliveryLicensed {
		t.Fatalf("delivery = %q, want %q", release.Delivery, PackageDeliveryLicensed)
	}
	if !release.RequiresVendorConfirmation {
		t.Fatal("licensed package must require vendor confirmation")
	}
	if release.SourceURL == "" || release.Version == "" || release.QueriedAt == "" {
		t.Fatal("licensed package must identify its source, version line, and query date")
	}
}

func TestReleaseForFlavorRejectsUnknownFlavor(t *testing.T) {
	if _, ok := ReleaseForFlavor("unknown"); ok {
		t.Fatal("unknown flavor must not have a release baseline")
	}
}
