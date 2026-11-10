package builders

import "strings"

type Select struct {
	Columns  Columns
	GroupBy  GroupByList
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

	if s.GroupBy.IsEmpty() == false {
		w.WriteString(" GROUP BY ")
		w.WriteString(s.GroupBy.Build())
	}

	return w.String()
}
