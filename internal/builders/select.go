package builders

import (
	"strings"

	"github.com/mineot/krios/internal/core"
)

type Select struct {
	Columns  []string
	GroupBy  []string
	Having   []string
	Joins    []string
	OrderBy  []string
	Paginate []string
	Tables   []Table
	Where    []string

	config core.Config
	errs   []error
}

func (s *Select) validate() bool {
	for _, t := range s.Tables {
		if t.Err() != nil {
			s.errs = append(s.errs, t.Err())
		}
	}

	return len(s.errs) == 0
}

func (s *Select) Errs() []error {
	return s.errs
}

func (s *Select) Build() string {
	if s.validate() == false {
		return ""
	}

	w := strings.Builder{}
	return w.String()
}
