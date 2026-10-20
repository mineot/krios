// Package write builds chained INSERT, UPDATE and DELETE statements from
// entity descriptors, renders them through the configured dialect and executes
// them on the DB or the transaction that created the builder. Construction is
// pure; rendering (Build) and execution surface every construction failure —
// including a RETURNING request the dialect does not support — before reaching
// the driver (E06-P01).
package write

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
)

// executor is the Krios execution contract satisfied by *DB and *Tx. It is
// declared here structurally so that write does not depend on the access or
// transaction packages; using it preserves the DB/Tx guards and the
// transaction policy of E03 unchanged.
type executor interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
	QueryContext(context.Context, string, ...any) (*scan.Rows, error)
	QueryRowContext(context.Context, string, ...any) scan.Row
}

// builder holds the state shared by Insert, Update and Delete (E06-P01 §3):
// the executor and dialect retained by the access, the base entity, the
// omitted columns, the RETURNING flag and the first construction failure.
type builder struct {
	exec      executor
	dialect   dialect.Dialect
	ent       *entity.Entity
	omit      []string
	returning bool
	err       error
}

// remember stores the first construction failure; later failures are dropped
// (first-wins). The stored error is returned by prepare on Build and before
// any execution.
func (b *builder) remember(err error) {
	if err != nil && b.err == nil {
		b.err = err
	}
}

// prepare runs the checks shared by every write statement and returns the set
// of omitted columns: the remembered failure, entity and dialect presence,
// Omit names (unknown or duplicated) and RETURNING support in the dialect.
func (b *builder) prepare(op string) (map[string]bool, error) {
	if b == nil || b.ent == nil {
		return nil, fmt.Errorf("%w: %s requires an entity", errs.ErrInvalidQuery, op)
	}

	if b.err != nil {
		return nil, b.err
	}

	if b.dialect == nil {
		return nil, fmt.Errorf("%w: %s requires a dialect", errs.ErrInvalidQuery, op)
	}

	omit, err := b.omitSet()

	if err != nil {
		return nil, err
	}

	if b.returning {
		rd, ok := b.dialect.(dialect.ReturningDialect)

		if !ok || !rd.SupportsReturning() {
			return nil, fmt.Errorf("%w: dialect %s does not support RETURNING", errs.ErrInvalidQuery, b.dialect.Name())
		}
	}

	return omit, nil
}

// omitSet validates the Omit names against the mapped columns and returns
// them as a set. Unknown and duplicated names fail with errs.ErrInvalidQuery.
func (b *builder) omitSet() (map[string]bool, error) {
	if len(b.omit) == 0 {
		return nil, nil
	}

	columns := b.ent.Columns()
	set := make(map[string]bool, len(b.omit))

	for _, name := range b.omit {
		if !slices.Contains(columns, name) {
			return nil, fmt.Errorf("%w: unknown column %q in Omit for table %q", errs.ErrInvalidQuery, name, b.ent.Table())
		}

		if set[name] {
			return nil, fmt.Errorf("%w: duplicate column %q in Omit", errs.ErrInvalidQuery, name)
		}

		set[name] = true
	}

	return set, nil
}

// renderReturning appends the RETURNING clause with every mapped column of
// the entity, in declaration order, when the statement requested it.
func (b *builder) renderReturning(w *strings.Builder) {
	if !b.returning {
		return
	}

	w.WriteString(" RETURNING ")

	for i, column := range b.ent.Columns() {
		if i > 0 {
			w.WriteString(", ")
		}

		w.WriteString(b.dialect.QuoteIdent(column))
	}
}

// noExecutor reports the missing executor with errs.ErrInvalidQuery.
func (b *builder) noExecutor(op string) error {
	if b.exec == nil {
		return fmt.Errorf("%w: %s has no executor", errs.ErrInvalidQuery, op)
	}

	return nil
}

// renderWhere appends the WHERE clause combining conditions with AND, or
// fails when no condition was given (E06-P01 §5: UPDATE and DELETE always
// require at least one condition; there is no "update everything" shortcut).
// Every referenced table must be the base table of the statement.
func renderWhere(wheres []entity.Condition, op string, b *builder, w *strings.Builder, args *[]any) error {
	if len(wheres) == 0 {
		return fmt.Errorf("%w: %s requires at least one Where condition", errs.ErrInvalidQuery, op)
	}

	table := b.ent.Table()
	w.WriteString(" WHERE ")

	for i, cond := range wheres {
		if i > 0 {
			w.WriteString(" AND ")
		}

		for _, ent := range cond.Entities() {
			if ent.Table() != table {
				return fmt.Errorf("%w: table %q is referenced but not part of the statement", errs.ErrInvalidQuery, ent.Table())
			}
		}

		fragment, err := cond.RenderInto(b.dialect, args)

		if err != nil {
			return err
		}

		w.WriteString(fragment)
	}

	return nil
}
