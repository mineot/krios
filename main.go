package main

import (
	"fmt"

	"github.com/mineot/krios/internal/table"
)

func main() {
	fmt.Println("Krios")

	t := table.Table{Name: "users", As: "u", Schema: "coorp"}
	fmt.Println(t.String())
}
