package expressions

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mineot/krios/internal/errs"
)

// Reference is an immutable identifier belonging to one client.
type Reference struct {
	owner    *Identity
	parts    []string
	wildcard bool
	err      error
}

func NewReference(owner *Identity, name string) *Reference {
	parts := strings.Split(name, ".")
	r := NewReferenceParts(owner, parts...)

	for i, part := range parts {
		if part == "*" {
			if i != len(parts)-1 {
				r.err = invalid("wildcard must be the final component")
			} else {
				r.wildcard = true
			}
		}
	}

	return r
}
func NewReferenceParts(owner *Identity, parts ...string) *Reference {
	r := &Reference{owner: owner, parts: slices.Clone(parts)}

	if !ValidIdentity(owner) {
		r.err = invalid("invalid operand origin")
	}

	if len(parts) == 0 {
		r.err = invalid("identifier requires components")
	}

	for _, part := range parts {
		if part == "" || strings.ContainsRune(part, 0) {
			r.err = invalid("invalid identifier component")
		}
	}

	return r
}
func validateReference(r *Reference, owner *Identity) error {
	if r == nil || !ValidIdentity(r.owner) || !ValidIdentity(owner) {
		return invalid("invalid reference")
	}

	if r.err != nil {
		return r.err
	}

	if r.owner != owner {
		return invalid("operand belongs to another client")
	}

	return nil
}

// ReadReference returns a snapshot; callers cannot mutate the stored components.
func ReadReference(r *Reference, owner *Identity) ([]string, bool, error) {
	if err := validateReference(r, owner); err != nil {
		return nil, false, err
	}

	return slices.Clone(r.parts), r.wildcard, nil
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", errs.ErrInvalidQuery, message)
}
