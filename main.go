package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	krios "github.com/mineot/krios/internal"
	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("Hello krios")

	db, err := sql.Open("sqlite", "file:./dev.db?mode=rw")

	if err != nil {
		fmt.Println(err)
		return
	}

	defer db.Close()

	client, err := krios.NewClient(
		krios.Config{
			DB:         db,
			DBVersion:  1,
			Dialect:    krios.SQLite,
			Timeout:    5 * time.Second,
			Context:    context.Background(),
			Migrations: nil,
		},
	)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(client)
}
