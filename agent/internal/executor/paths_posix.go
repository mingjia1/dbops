package executor

import (
	"path"
	"path/filepath"
	"strings"
)

// posixPathUnder reports whether p is an absolute POSIX path (leading "/")
// located strictly under root (or at root itself when allowRoot is true),
// with no ".." traversal. The agent only deploys to Linux hosts, so path
// policy uses POSIX semantics regardless of the OS the agent binary runs on;
// filepath.IsAbs/filepath.Clean would wrongly reject "/opt/..." paths when
// tests or tooling run on Windows.
func posixPathUnder(p, root string, allowRoot bool) bool {
	if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return false
	}
	root = strings.TrimSuffix(root, "/")
	cleaned := path.Clean(p)
	if cleaned == root {
		return allowRoot
	}
	return strings.HasPrefix(cleaned, root+"/")
}

// absolutePath reports whether p is absolute under POSIX or host-OS rules.
// Linux agents satisfy both for "/..." paths; the host-OS branch only matters
// when tests run on Windows (e.g. t.TempDir() paths).
func absolutePath(p string) bool {
	return filepath.IsAbs(p) || strings.HasPrefix(p, "/")
}
