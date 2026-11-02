package expression

import (
	"fmt"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// ValidateOrderCapabilities checks support after structural validation.
func ValidateOrderCapabilities(order OrderTerm, info dialect.Info) error {
	if order.nulls != nullsDefault && !info.Capabilities.ExplicitNullOrder {
		return fmt.Errorf("%w: explicit NULL ordering", errs.ErrUnsupported)
	}
	return nil
}
