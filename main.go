package main

import (
	"fmt"

	krios "github.com/mineot/krios/internal"
	selectbuilder "github.com/mineot/krios/internal/builder/select"
)

func main() {
	fmt.Println("Hello krios")

	b := selectbuilder.Select{
		Columns: []krios.Column{
			{Name: "id", As: "u_id", Table: "u"},
			{Name: "name", As: "u_name", Table: "u"},
			{Name: "age", As: "u_age", Table: "u"},
			{Name: "val1", As: "count", Table: "u", Aggregator: krios.Count},
			{Name: "val2", As: "avg", Table: "u", Aggregator: krios.Avg},
			{Name: "val3", As: "max", Table: "u", Aggregator: krios.Max},
			{Name: "val4", As: "min", Table: "u", Aggregator: krios.Min},
			{Name: "val5", As: "sum", Table: "u", Aggregator: krios.Sum},
		},
		Tables: []krios.Table{
			{Name: "users", As: "u", Schema: "public"},
		},
	}

	if b.GetError() != nil {
		panic(b.GetError())
	}

	fmt.Println("postgresql:", b.Build(krios.Postgres))
	fmt.Println("mysql:", b.Build(krios.Mysql))
	fmt.Println("sqlite3: ", b.Build(krios.SQLite))
}
