package builders

import (
	"strings"
)

type SelectBuilder struct {
	Columns  []string
	GroupBy  []string
	Having   []string
	Joins    []string
	OrderBy  []string
	Paginate []string
	Tables   []TableBuilder
	Where    []string

	config ConfigBuilder
	errs   []error
}

func (s *SelectBuilder) validate() bool {
	for _, t := range s.Tables {
		if t.Err() != nil {
			s.errs = append(s.errs, t.Err())
		}
	}

	return len(s.errs) == 0
}

func (s *SelectBuilder) SetConfig(config ConfigBuilder) {
	s.config = config
}

func (s *SelectBuilder) Errs() []error {
	return s.errs
}

func (s *SelectBuilder) Build() string {
	if s.validate() == false {
		return ""
	}

	w := strings.Builder{}
	return w.String()
}
