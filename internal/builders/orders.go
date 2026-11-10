package builders

import (
	"strings"
)

type OrderByList []OrderBy

func (o *OrderByList) IsEmpty() bool {
	return len(*o) == 0
}

func (o *OrderByList) Add(or OrderBy) {
	*o = append(*o, or)
}

func (o *OrderByList) Validate() error {
	for _, or := range *o {
		if err := or.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (o *OrderByList) Build() string {
	var ors []string

	for _, or := range *o {
		ors = append(ors, or.Build())
	}

	return strings.Join(ors, ", ")
}
