package expression

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mineot/krios/internal/errs"
)

func invalid(description string) error {
	return fmt.Errorf("%w: %s", errs.ErrInvalidQuery, description)
}

func validateComponent(name string) error {
	if !utf8.ValidString(name) || strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\x00.*") {
		return invalid("invalid identifier component")
	}

	return nil
}

// validateColumn validates qualification without modifying any component.
// It reports whether the final component is an allowed projection wildcard.
func validateColumn(name string, allowWildcard bool) (bool, error) {
	parts := strings.Split(name, ".")

	for i, part := range parts {
		if allowWildcard && part == "*" && i == len(parts)-1 {
			return true, nil
		}

		if err := validateComponent(part); err != nil {
			return false, fmt.Errorf("column component %d: %w", i, err)
		}
	}

	return false, nil
}
