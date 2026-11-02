package selectquery

import (
	"database/sql"

	"github.com/mineot/krios/internal/compiler"
	"github.com/mineot/krios/internal/expression"
)

// Build compiles against captured metadata without checking terminal Tx state.
func (q Builder) Build() (string, []any, error) {
	if q.tx == nil {
		return "", nil, sql.ErrTxDone
	}
	if q.initErr != nil {
		return "", nil, q.initErr
	}
	w, err := compiler.New(q.info)
	if err != nil {
		return "", nil, err
	}
	if err := q.validate(); err != nil {
		return "", nil, err
	}
	w.Syntax("SELECT ")
	if q.distinct {
		w.Syntax("DISTINCT ")
	}
	if !q.columnsSet {
		w.Syntax("*")
	} else {
		for i, column := range q.columns {
			if i > 0 {
				w.Syntax(", ")
			}
			expression.WriteExpr(w, column, expression.Projection)
		}
	}
	w.Syntax(" FROM ")
	expression.WriteTable(w, q.table)
	if len(q.where) != 0 {
		w.Syntax(" WHERE ")
		expression.WritePredicate(w, q.condition())
	}
	if len(q.order) != 0 {
		w.Syntax(" ORDER BY ")
		for i, term := range q.order {
			if i > 0 {
				w.Syntax(", ")
			}
			expression.WriteOrder(w, term)
		}
	}
	return w.Result()
}
