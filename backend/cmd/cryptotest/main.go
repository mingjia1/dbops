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
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: decrypt_test <encrypt|decrypt> <text>\n")
		os.Exit(1)
	}
	action := os.Args[1]
	text := os.Args[2]
	switch action {
	case "encrypt":
		encrypted, err := utils.Encrypt(text, encKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "encrypt error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(encrypted)
	case "decrypt":
		decrypted, err := utils.Decrypt(text, encKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decrypt error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(decrypted)
	}
}
