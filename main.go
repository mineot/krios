package main

import (
	"fmt"

	"github.com/mineot/krios/internal/builders/table"
	"github.com/mineot/krios/internal/dialects"
)

func main() {
	fmt.Println("Hello krios")

	t := table.Table{Name: "table", Dialect: dialects.Postgres}
	t.As("t")
	t.Schema("schema")
	fmt.Println(t.Build())
}
