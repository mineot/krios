package builders

import "github.com/mineot/krios/internal/core"

type Column struct {
	Aggregate Aggregate
	As        string
	Config    core.Config
	Name      string
	Table     string

	err error
}
