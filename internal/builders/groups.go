package builders

import (
	"strings"
)

type GroupByList []GroupBy

func (g *GroupByList) IsEmpty() bool {
	return len(*g) == 0
}

func (g *GroupByList) Add(gp GroupBy) {
	*g = append(*g, gp)
}

func (g *GroupByList) Validate() error {
	for _, tb := range *g {
		if err := tb.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (g *GroupByList) Build() string {
	var gps []string

	for _, gp := range *g {
		gps = append(gps, gp.Build())
	}

	return strings.Join(gps, ", ")
}
