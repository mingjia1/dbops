package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configInt(config map[string]interface{}, key string) int {
	if v, ok := config[key].(int); ok {
		return v
	}
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	return 0
}

func configString(config map[string]interface{}, key string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return ""
}

func configBool(config map[string]interface{}, key string) bool {
	if v, ok := config[key].(bool); ok {
		return v
	}
	if v, ok := config[key].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "on":
			return true
		}
	}
	return false
}

func configPath(config map[string]interface{}) string {
	path, _ := config["path"].(string)
	if path == "" {
		return "/etc/my.cnf"
	}
	return path
}

func configPathCandidates(config map[string]interface{}) []string {
	candidates := make([]string, 0, 8)
	add := func(path string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			return
		}
		for _, existing := range candidates {
			if existing == path {
				return
			}
		}
		candidates = append(candidates, path)
	}
	if port := configInt(config, "target_port"); port > 0 {
		add(fmt.Sprintf("/etc/dbops-pxc/dbops-pxc-%d.cnf", port))
		add(fmt.Sprintf("/tmp/pxc-%d.cnf", port))
	}
	if datadir, _ := config["datadir"].(string); strings.TrimSpace(datadir) != "" {
		add(filepath.Join(datadir, "my.cnf"))
		add(filepath.Join(datadir, "mysql.cnf"))
	}
	add("/etc/mha/app1.cnf")
	add("/etc/my.cnf")
	add("/etc/mysql/my.cnf")
	return candidates
}

func resolveConfigPath(config map[string]interface{}) (string, []string) {
	rawRequested := strings.TrimSpace(configPath(config))
	requested := filepath.Clean(rawRequested)
	if requested != "." && requested != "" && filepath.ToSlash(requested) != "/etc/my.cnf" && rawRequested != "/etc/my.cnf" {
		if st, err := os.Stat(requested); err == nil && !st.IsDir() {
			return requested, []string{requested}
		}
		return requested, []string{requested}
	}
	candidates := configPathCandidates(config)
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, candidates
		}
	}
	return "", candidates
}

func escapeSQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

func defaultString(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

func backupMySQLHostCandidates(host string) []string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	candidates := []string{host}
	if isLocalHost(host) {
		candidates = append(candidates, "127.0.0.1", "localhost")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}
	}
	return out
}
