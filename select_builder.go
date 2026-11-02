package krios

import "github.com/mineot/krios/internal/selectquery"

// SelectBuilder is an immutable query obtained through Tx.Select.
// Reassign or chain modifiers. Its zero value cannot build or execute a query.
type SelectBuilder struct {
	query selectquery.Builder
}
