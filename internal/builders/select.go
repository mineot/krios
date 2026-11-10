package builders

import "strings"

type Select struct {
	Columns  Columns
	GroupBy  GroupByList
	Having   []string
	Joins    []string
	OrderBy  OrderByList
	Paginate Paginate
	Tables   Tables
	Where    []string
}

func (s *Select) Validate() error {
	if err := s.Columns.Validate(); err != nil {
		return err
	}

	if err := s.Tables.Validate(); err != nil {
		return err
	}

	if err := s.GroupBy.Validate(); err != nil {
		return err
	}

	if err := s.OrderBy.Validate(); err != nil {
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

	if len(s.OrderBy) > 0 {
		w.WriteString(" ORDER BY ")
		w.WriteString(s.OrderBy.Build())
	}

	if s.Paginate.IsEmpty() == false {
		w.WriteString(s.Paginate.Build())
	}

	return w.String()
}
