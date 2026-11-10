package builders

import "strings"

type Select struct {
	Columns  Columns
	GroupBy  []string
	Having   []string
	Joins    []string
	OrderBy  []string
	Paginate []string
	Tables   Tables
	Where    []string
}

func (s *Select) Validate() error {
	if err := s.Tables.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *Select) Build() string {
	w := strings.Builder{}
	w.WriteString("SELECT ")
	w.WriteString(s.Columns.Build())
	w.WriteString(" FROM ")
	w.WriteString(s.Tables.Build())
	return w.String()
}
