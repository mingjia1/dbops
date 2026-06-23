package repositories

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// JSONStore provides a simple file-based JSON persistence for the in-memory
// repositories. This allows the standalone (no-MySQL) mode to survive
// restarts, matching the user's expectation that CRUD operations persist.
//
// On startup each repository loads from its dedicated file under DataDir.
// On every mutation we write atomically (temp file + rename) to avoid
// corrupting the file on a crash mid-write.
type JSONStore struct {
	dir string
	mu  sync.Mutex
}

func NewJSONStore(dir string) (*JSONStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("json store dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data dir %s: %w", dir, err)
	}
	return &JSONStore{dir: dir}, nil
}

func (s *JSONStore) load(name string, v interface{}) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return nil
}

func (s *JSONStore) save(name string, v interface{}) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", name, err)
	}
	path := filepath.Join(s.dir, name+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to rename %s: %w", path, err)
	}
	return nil
}
