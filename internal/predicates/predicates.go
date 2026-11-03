package predicates

import (
	"fmt"
	"time"
)

type Predicate struct {
	Column   string
	Operator string
	Value    string
}

func Eq(column string, value any) Predicate {
	return make(column, value, "=")
}

func make(column string, value any, operator string) Predicate {
	return Predicate{
		Column:   column,
		Operator: operator,
		Value:    normalizeValue(value),
	}
}

func normalizeValue(value any) string {
	if valstr, ok := value.(string); ok {
		return "'" + valstr + "'"
	} else if valtime, ok := value.(time.Time); ok {
		return "'" + valtime.Format(time.RFC3339) + "'"
	}

	return fmt.Sprint(value)
}

// krios.Ne("id", 2),
// krios.Lt("id", 10),
// krios.Lte("id", 10),
// krios.Gt("id", 1),
// krios.Gte("id", 1),
// krios.In("id", 1, 2, 3),
// krios.NotIn("id", 4, 5),
// krios.Like("name", "Jo%"),
// krios.NotLike("name", "%test%"),
// krios.IsNull("deleted_at"),
// krios.IsNotNull("created_at"),
// krios.Between("id", 1, 10),
// krios.NotBetween("id", 20, 30),
// krios.Gte("created_at", time.Now().UTC().Add(-24*time.Hour)),
