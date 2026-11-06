package main

import (
	"fmt"

	"github.com/mineot/krios/internal/builders/selects"
	"github.com/mineot/krios/internal/dialects"
)

func main() {
	fmt.Println("Hello krios")

	s := selects.NewSelect(dialects.Postgres)
	s.From("table")

	sql := s.Build()

	if err := s.Err(); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(sql)
}
