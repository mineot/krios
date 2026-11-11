package core

import "github.com/mineot/krios/internal/builders"

type Criteria struct {
	Column   string
	Operator builders.Operator
	Or       []Criteria
	Table    string
	Value    any
}

type Paginate struct {
	Limit  uint16
	Offset uint16
}

type OrderBy struct {
	Column    string
	Direction builders.Direction
	Table     string
}

type GroupBy struct {
	Column string
	Table  string
}

type Column struct {
	Aggregate builders.Aggregate
	As        string
	Name      string
	Table     string
}

type Table struct {
	As     string
	Name   string
	Schema string
}

type Select struct {
	Columns  []Column
	GroupBy  []GroupBy
	Having   []string
	Joins    []string
	OrderBy  []OrderBy
	Paginate Paginate
	Tables   []Table
	Where    []Criteria
}

func (c *Client) Select(s Select) (*builders.Select, error) {
	s1 := builders.Select{}

	var config = builders.Config{
		DB:      c.Config.DB,
		Dialect: c.Config.Dialect,
		Context: c.Config.Context,
	}

	for _, cl := range s.Columns {
		s1.Columns.Add(
			builders.Column{
				Aggregate: cl.Aggregate,
				As:        cl.As,
				Config:    config,
				Name:      cl.Name,
				Table:     cl.Table,
			},
		)
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

	for index, wh := range s.Where {
		s1.Where.Add(
			builders.Criteria{
				Column:   wh.Column,
				Config:   config,
				Index:    index,
				Operator: wh.Operator,
				Table:    wh.Table,
				Value:    wh.Value,
			},
		)
	}

	for _, gp := range s.GroupBy {
		s1.GroupBy.Add(
			builders.GroupBy{
				Column: gp.Column,
				Config: config,
				Table:  gp.Table,
			},
		)
	}

	for _, or := range s.OrderBy {
		s1.OrderBy.Add(
			builders.OrderBy{
				Column:    or.Column,
				Config:    config,
				Direction: or.Direction,
				Table:     or.Table,
			},
		)
	}

	if s.Paginate != (Paginate{}) {
		s1.Paginate = builders.Paginate{
			Limit:  s.Paginate.Limit,
			Offset: s.Paginate.Offset,
		}
	}

	if err := s1.Validate(); err != nil {
		return nil, err
	}

	return &s1, nil
}
