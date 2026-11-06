package selects

import "github.com/mineot/krios/internal/dialects"

type From struct {
	Dialect dialects.Dialect
	Name    string
	as      string
	schema  string
	err     error
}
