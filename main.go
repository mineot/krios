package main

import (
	"fmt"

	"github.com/mineot/krios/internal/builders/selects"
)

func main() {
	fmt.Println("Hello krios")

	s := selects.Select{}
	s.From("users").As("u").Schema("public")
}
