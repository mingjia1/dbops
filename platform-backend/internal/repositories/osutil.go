package repositories

import "os"

// osMkdirAll 拆出便于测试, 避免 database.go 直接依赖 os
func osMkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
