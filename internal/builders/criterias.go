package builders

import "strings"

type Criterias []Criteria

func (c *Criterias) IsEmpty() bool {
	return len(*c) == 0
}

func (c *Criterias) Add(cr Criteria) {
	*c = append(*c, cr)
}

func (c *Criterias) Validate() error {
	for _, cr := range *c {
		if err := cr.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Criterias) Build() string {
	var crs []string

	for _, cr := range *c {
		crs = append(crs, cr.build())
	}

	return strings.Join(crs, " AND ")
}

func (c *Criterias) Values() []any {
	var vs []any

	for _, cr := range *c {
		vs = append(vs, cr.Values()...)
	}

	return vs
}
