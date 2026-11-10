package builders

import (
	"github.com/mineot/krios/internal/core"
)

type Select struct {
	Columns  []string
	Config   core.Config
	GroupBy  []string
	Having   []string
	Joins    []string
	OrderBy  []string
	Paginate []string
	Tables   []Table
	Where    []string

	errors []error
}
