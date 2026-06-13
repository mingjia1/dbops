package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "data/dbops.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// 查询users表结构
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== Users Table Schema ===")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			log.Printf("Scan failed: %v", err)
			continue
		}

		dflt := "NULL"
		if dfltValue.Valid {
			dflt = dfltValue.String
		}

		fmt.Printf("%d. %s (%s) NotNull:%d Default:%s PK:%d\n",
			cid, name, ctype, notnull, dflt, pk)
	}
}
