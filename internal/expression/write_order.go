package expression

import "github.com/mineot/krios/internal/compiler"

// WriteOrder validates structure before checking support for explicit NULL order.
func WriteOrder(w *compiler.Writer, order OrderTerm) {
	if w.Err() != nil {
		return
	}

	if err := ValidateOrder(order); err != nil {
		w.Fail(err)
		return
	}

	if err := ValidateOrderCapabilities(order, w.Info()); err != nil {
		w.Fail(err)
		return
	}

	writeColumn(w, order.column)

	if order.direction == ascending {
		w.Syntax(" ASC")
	} else {
		w.Syntax(" DESC")
	}

	switch order.nulls {
	case nullsFirst:
		w.Syntax(" NULLS FIRST")
	case nullsLast:
		w.Syntax(" NULLS LAST")
	}
}
