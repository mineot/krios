// Package selectquery builds immutable SELECT queries bound to a transaction.
package selectquery

import (
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/expression"
	"github.com/mineot/krios/internal/transaction"
)

// Builder keeps private query state. Modifiers return independent versions.
type Builder struct {
	tx         *transaction.Tx
	info       dialect.Info
	initErr    error
	table      expression.TableRef
	distinct   bool
	columnsSet bool
	columns    []expression.Expr
	where      []expression.Predicate
	order      []expression.OrderTerm
}

// New captures compilation metadata without requiring a live transaction.
func New(tx *transaction.Tx, table expression.TableRef) Builder {
	info, err := tx.CompilationInfo()
	return Builder{tx: tx, info: info, initErr: err, table: table}
}
