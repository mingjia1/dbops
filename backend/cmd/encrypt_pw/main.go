package main

import (
	"fmt"
	"os"

	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

func main() {
	encKey := os.Getenv("DBOPS_ENCRYPTION_KEY")
	if encKey == "" {
		encKey = "test-encryption-key-for-testing-at-least-32"
	}
	password := os.Args[1]
	encrypted, err := utils.Encrypt(password, encKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(encrypted)
}
