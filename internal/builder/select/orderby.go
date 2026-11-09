package selectbuilder

import (
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

func (s *Select) hasOrders() bool {
	return len(s.Orders) > 0
}

func (s *Select) getOrders(d dialects.Dialect) string {
	orders := []string{}

	for _, o := range s.Orders {
		o1 := o.Build(d)

		if o.GetError() != nil {
			s.appendError(o.GetError())
			return ""
		}

		orders = append(orders, o1)
	}

	return strings.Join(orders, ", ")
}
