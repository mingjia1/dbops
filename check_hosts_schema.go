package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "d:/test_tmple/new_dbops/dbops/platform-backend/data/dbops.db")
	defer db.Close()

	fmt.Println("=== hosts table schema ===")
	rows, _ := db.Query("PRAGMA table_info(hosts)")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		fmt.Printf("  %s (%s)\n", name, ctype)
	}
	rows.Close()

	fmt.Println("\n=== All hosts ===")
	rows2, _ := db.Query("SELECT * FROM hosts LIMIT 5")
	cols, _ := rows2.Columns()
	fmt.Printf("Columns: %v\n", cols)

	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	for rows2.Next() {
		rows2.Scan(valPtrs...)
		fmt.Println("\n---")
		for i, col := range cols {
			fmt.Printf("%s: %v\n", col, vals[i])
		}
	}
	rows2.Close()
}
