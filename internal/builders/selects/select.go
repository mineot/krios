// Package selects holds the internal state for SELECT query construction.
package selects

// Select stores the clause fragments of a SELECT query.
type Select struct {
	columns   []string
	from      string
	joins     []string
	criterias []string
	orders    []string
	groups    []string
	paginate  []string
}
