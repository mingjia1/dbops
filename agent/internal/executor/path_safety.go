package executor

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SafeJoin(baseDir, fileName string) (string, error) {
	fullPath := filepath.Join(baseDir, fileName)
	cleanBase := filepath.Clean(baseDir)
	cleanFull := filepath.Clean(fullPath)

	if !strings.HasPrefix(cleanFull, cleanBase+string(filepath.Separator)) && cleanFull != cleanBase {
		return "", fmt.Errorf("path traversal detected: %q escapes base %q", fileName, baseDir)
	}
	return cleanFull, nil
}
