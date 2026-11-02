package krios

import (
	"github.com/mineot/krios/internal/selectquery"
	"github.com/mineot/krios/internal/transaction"
)

// Select creates an immutable query bound to this transaction.
// A previously initialized, finished Tx can still construct and build queries;
// execution requires an active Tx. Nil or uninitialized Tx produces an invalid builder.
func (tx *Tx) Select(table TableRef) SelectBuilder {
	var inner *transaction.Tx
	if tx != nil {
		inner = tx.transaction
	}
	return SelectBuilder{query: selectquery.New(inner, table)}
}
