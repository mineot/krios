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
	fmt.Println()

	s1, _ := client.Select(
		krios.Select{
			// Columns: []krios.Column{
			// 	// {Name: "id", As: "id", Table: "u"},
			// 	// {Name: "name", As: "name", Table: "u"},
			// 	// {Name: "email", As: "email", Table: "u"},
			// 	// {Name: "val1", As: "sum1", Table: "u", Aggregate: krios.Sum},
			// 	// {Name: "val2", As: "avg1", Table: "u", Aggregate: krios.Avg},
			// 	// {Name: "val3", As: "min1", Table: "u", Aggregate: krios.Min},
			// 	// {Name: "val4", As: "max1", Table: "u", Aggregate: krios.Max},
			// 	// {Name: "val5", As: "count1", Table: "u", Aggregate: krios.Count},
			// 	// {Name: "*", As: "total", Table: "u", Aggregate: krios.Count},
			// },
			Tables: []krios.Table{
				{Name: "user", As: "u", Schema: "public"},
			},
			GroupBy: []krios.GroupBy{
				{Column: "id", Table: "u"},
				{Column: "name"},
			},
		},
	)

	fmt.Println(s1.Build())
}
