package main

import (
	"fmt"

	"github.com/mineot/krios/internal/builder"
	selectbuilder "github.com/mineot/krios/internal/builder/select"
	"github.com/mineot/krios/internal/dialects"
)

func main() {
	fmt.Println("Hello krios")

	b := selectbuilder.Select{
		Columns: []builder.Column{
			{Name: "id", As: "u_id", Table: "u"},
			{Name: "name", As: "u_name", Table: "u"},
			{Name: "age", As: "u_age", Table: "u"},
			{Name: "val1", As: "count", Table: "u", Aggregator: builder.CountAggregator},
			{Name: "val2", As: "avg", Table: "u", Aggregator: builder.AvgAggregator},
			{Name: "val3", As: "max", Table: "u", Aggregator: builder.MaxAggregator},
			{Name: "val4", As: "min", Table: "u", Aggregator: builder.MinAggregator},
			{Name: "val5", As: "sum", Table: "u", Aggregator: builder.SumAggregator},
		},
		Tables: []builder.Table{
			{Name: "users", As: "u", Schema: "public"},
		},
	}

	if b.GetError() != nil {
		panic(b.GetError())
	}

	fmt.Println("postgresql:", b.Build(dialects.PostgresDialect))
	fmt.Println("mysql:", b.Build(dialects.MysqlDialect))
	fmt.Println("sqlite3: ", b.Build(dialects.SQLiteDialect))
}
