package selectbuilder

import (
	"strings"

	"github.com/mineot/krios/internal/dialects"
)

func (s *Select) hasGroups() bool {
	return len(s.Groups) > 0
}

func (s *Select) getGroups(d dialects.Dialect) string {
	groups := []string{}

	for _, g := range s.Groups {
		g1 := g.Build(d)

		if g.GetError() != nil {
			s.appendError(g.GetError())
			return ""
		}

		groups = append(groups, g1)
	}

	return strings.Join(groups, ", ")
}
