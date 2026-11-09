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
			{Name: "val1", As: "count", Table: "u", Aggregator: krios.CountAggregator},
			{Name: "val2", As: "avg", Table: "u", Aggregator: krios.AvgAggregator},
			{Name: "val3", As: "max", Table: "u", Aggregator: krios.MaxAggregator},
			{Name: "val4", As: "min", Table: "u", Aggregator: krios.MinAggregator},
			{Name: "val5", As: "sum", Table: "u", Aggregator: krios.SumAggregator},
		},
		Tables: []krios.Table{
			{Name: "users", As: "u", Schema: "public"},
		},
	}

	if b.GetError() != nil {
		panic(b.GetError())
	}

	fmt.Println("postgresql:", b.Build(krios.PostgresDialect))
	fmt.Println("mysql:", b.Build(krios.MysqlDialect))
	fmt.Println("sqlite3: ", b.Build(krios.SQLiteDialect))
}
