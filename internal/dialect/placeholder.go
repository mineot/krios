package dialect

import (
	"fmt"
	"strconv"

	"github.com/mineot/krios/internal/errs"
)

// Placeholder returns a positional placeholder for a one-based argument index.
func Placeholder(kind Dialect, position int) (string, error) {
	if !Valid(kind) {
		return "", fmt.Errorf("%w: unknown dialect", errs.ErrInvalidConfig)
	}

	if position < 1 {
		return "", fmt.Errorf("%w: invalid parameter position", errs.ErrInvalidQuery)
	}

	if kind == Postgres {
		return "$" + strconv.Itoa(position), nil
	}

	return "?", nil
}
