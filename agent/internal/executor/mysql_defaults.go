package executor

import (
	"fmt"
	"os"
	"path/filepath"
)

type MySQLDefaultsFile struct {
	path string
}

func NewMySQLDefaultsFile(host string, port int, user, password string) (*MySQLDefaultsFile, error) {
	if port == 0 {
		port = 3306
	}
	if user == "" {
		user = "root"
	}

	content := fmt.Sprintf(`[client]
host=%s
port=%d
user=%s
password=%s
`, host, port, user, password)

	dir := os.TempDir()
	name := fmt.Sprintf("mysql_defaults_%d_%d.cnf", os.Getpid(), port)
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return nil, fmt.Errorf("write defaults file: %w", err)
	}

	return &MySQLDefaultsFile{path: path}, nil
}

func (f *MySQLDefaultsFile) Path() string {
	return f.path
}

func (f *MySQLDefaultsFile) Cleanup() {
	if f.path != "" {
		os.Remove(f.path)
	}
}

func (f *MySQLDefaultsFile) Args() []string {
	return []string{"--defaults-extra-file=" + f.path}
}
