package core

import "github.com/mineot/krios/internal/builders"

type Table struct {
	As     string
	Name   string
	Schema string
}

type Select struct {
	Columns  []string
	GroupBy  []string
	Having   []string
	Joins    []string
	OrderBy  []string
	Paginate []string
	Tables   []Table
	Where    []string
}

func (c *Client) Select(s Select) (*builders.Select, error) {
	s1 := builders.Select{}

	var config = builders.Config{
		DB:      c.Config.DB,
		Dialect: c.Config.Dialect,
		Context: c.Config.Context,
	}

	for _, tb := range s.Tables {
		s1.Tables.Add(
			builders.Table{
				As:     tb.As,
				Config: config,
				Name:   tb.Name,
				Schema: tb.Schema,
			},
		)
	}

	if err := s1.Validate(); err != nil {
		return nil, err
	}

	return &s1, nil
}
