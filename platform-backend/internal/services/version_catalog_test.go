package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCatalogActiveEntriesHaveUsablePackageMetadata(t *testing.T) {
	for _, entry := range NewVersionCatalog().List() {
		if entry.Status != "active" {
			continue
		}
		assert.NotEmpty(t, entry.PackageURL, entry.ID)
		assert.True(t, strings.HasPrefix(entry.PackageURL, "https://"), entry.ID)
		assert.True(t, isSHA256Hex(entry.Checksum), entry.ID)
		assert.NotContains(t, strings.ToLower(entry.Checksum), "placeholder", entry.ID)
	}
}
