package expression

import (
	"fmt"

	"github.com/mineot/krios/internal/compiler"
	"github.com/mineot/krios/internal/errs"
)

// WriteOrder validates structure before checking support for explicit NULL order.
func WriteOrder(w *compiler.Writer, order OrderTerm) {
	if w.Err() != nil {
		return
	}

	if err := ValidateOrder(order); err != nil {
		w.Fail(err)
		return
	}

	if order.nulls != nullsDefault && !w.Info().Capabilities.ExplicitNullOrder {
		w.Fail(fmt.Errorf("%w: explicit NULL ordering", errs.ErrUnsupported))
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
