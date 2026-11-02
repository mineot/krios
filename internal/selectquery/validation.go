package selectquery

import (
	"fmt"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expression"
)

// validate checks all structural clauses before any capability requirement.
func (q Builder) validate() error {
	if err := expression.ValidateTable(q.table); err != nil {
		return err
	}
	if q.columnsSet && len(q.columns) == 0 {
		return fmt.Errorf("%w: empty SELECT projection", errs.ErrInvalidQuery)
	}
	for i, column := range q.columns {
		if err := expression.ValidateExpr(column, expression.Projection); err != nil {
			return fmt.Errorf("SELECT projection %d: %w", i, err)
		}
	}
	for i, group := range q.where {
		if err := expression.ValidatePredicate(group); err != nil {
			return fmt.Errorf("SELECT WHERE group %d: %w", i, err)
		}
	}
	for i, term := range q.order {
		if err := expression.ValidateOrder(term); err != nil {
			return fmt.Errorf("SELECT ordering term %d: %w", i, err)
		}
	}
	for _, term := range q.order {
		if err := expression.ValidateOrderCapabilities(term, q.info); err != nil {
			return err
		}
	}
	return nil
}
