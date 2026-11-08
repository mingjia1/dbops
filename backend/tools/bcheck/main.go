package main

import (
	"database/sql"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "../db/dbops.db")
	if err != nil { fmt.Println("open:", err); os.Exit(1) }
	var hash string
	if err := db.QueryRow("SELECT password FROM users WHERE username='admin'").Scan(&hash); err != nil {
		fmt.Println("query:", err); os.Exit(1)
	}
	fmt.Println("hash prefix:", hash[:10], "len:", len(hash))
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin123"))
	fmt.Println("admin123 match:", err == nil, "err:", err)
}
