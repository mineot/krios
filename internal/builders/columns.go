package builders

import (
	"strings"
)

type Columns []Column

func (c *Columns) Add(cl Column) {
	*c = append(*c, cl)
}

func (c *Columns) Validate() error {
	for _, cl := range *c {
		if err := cl.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Columns) Build() string {
	if len(*c) == 0 {
		return "*"
	}

	var cls []string

	for _, cl := range *c {
		cls = append(cls, cl.Build())
	}

	return strings.Join(cls, ", ")
}
