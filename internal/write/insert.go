package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
)

// Insert is a chained INSERT builder for one base entity. It is created by
// DB.Insert or Tx.Insert, is mutable in place (one builder per statement; not
// safe for concurrent reuse) and executes through the DB or Tx that created
// it. An INSERT represents a single row (E06-P01 §2); multi-row inserts are a
// registered future extension.
type Insert struct {
	builder
	values []entity.WriteValue
}

// NewInsert creates a builder bound to exec and rendering with d for the base
// entity ent.
func NewInsert(exec executor, d dialect.Dialect, ent *entity.Entity) *Insert {
	return &Insert{builder: builder{exec: exec, dialect: d, ent: ent}}
}

// SetValues assigns the row to insert. record must be T or *T of the entity's
// type. The values are extracted and the field validators of the entity run at
// call time (E06-P01 §8); failures are remembered and reported by Build and by
// the execution methods before the statement reaches the driver. A later call
// replaces the row of an earlier successful call; the first failure wins. The
// row is a snapshot: later changes to record do not affect the statement.
//
// The write policy of E06-P01 §4.2 applies: nil pointers bind NULL, omitted
// columns (see Omit) use the database default.
func (b *Insert) SetValues(record any) *Insert {
	if b == nil {
		return b
	}

	if b.ent == nil {
		b.remember(fmt.Errorf("%w: insert requires an entity", errs.ErrInvalidQuery))
		return b
	}

	if err := b.ent.Validate(record); err != nil {
		b.remember(err)
		return b
	}

	values, err := b.ent.WriteValues(record)

	if err != nil {
		b.remember(err)
		return b
	}

	b.values = values
	return b
}

// Omit excludes the named columns from the INSERT so the database default
// applies. Unknown or duplicated names are reported by Build with
// errs.ErrInvalidQuery. Repeated calls accumulate.
func (b *Insert) Omit(names ...string) *Insert {
	b.omit = append(b.omit, names...)
	return b
}

// Returning requests the inserted row back through the RETURNING clause.
// Without it, Exec returns the execution result; with it, One fills the
// destination. The dialect must support RETURNING (SQLite and PostgreSQL;
// MySQL fails on Build before executing — E06-P01 §7).
func (b *Insert) Returning() *Insert {
	b.returning = true
	return b
}

// Build renders the statement and its bound parameters without executing it,
// in textual order: column list, values, then RETURNING. Construction
// failures (missing entity or SetValues, no column to write, invalid Omit,
// unsupported RETURNING, remembered SetValues failure) are reported with
// errs.ErrInvalidQuery, errs.ErrInvalidEntity or errs.ErrValidation.
func (b *Insert) Build() (string, []any, error) {
	if b == nil {
		return "", nil, fmt.Errorf("%w: insert requires an entity", errs.ErrInvalidQuery)
	}

	omit, err := b.prepare("insert")

	if err != nil {
		return "", nil, err
	}

	if b.values == nil {
		return "", nil, fmt.Errorf("%w: insert requires SetValues", errs.ErrInvalidQuery)
	}

	cols := make([]entity.WriteValue, 0, len(b.values))

	for _, value := range b.values {
		if omit[value.Column] {
			continue
		}

		cols = append(cols, value)
	}

	if len(cols) == 0 {
		return "", nil, fmt.Errorf("%w: insert has no columns to write", errs.ErrInvalidQuery)
	}

	var (
		w    strings.Builder
		args []any
	)

	w.WriteString("INSERT INTO ")
	w.WriteString(b.dialect.QuoteIdent(b.ent.Table()))
	w.WriteString(" (")

	for i, col := range cols {
		if i > 0 {
			w.WriteString(", ")
		}

		w.WriteString(b.dialect.QuoteIdent(col.Column))
	}

	w.WriteString(") VALUES (")

	for i, col := range cols {
		if i > 0 {
			w.WriteString(", ")
		}

		args = append(args, col.Value)
		w.WriteString(b.dialect.Placeholder(len(args)))
	}

	w.WriteByte(')')
	b.renderReturning(&w)

	return w.String(), args, nil
}

// Exec executes the insert and returns the execution information
// (krios.Result with RowsAffected and LastInsertId). It is rejected after
// Returning, where One must be used instead. Construction failures and DB/Tx
// guards are applied before execution.
func (b *Insert) Exec(ctx context.Context) (result.Result, error) {
	query, args, err := b.Build()

	if err != nil {
		return nil, err
	}

	if b.returning {
		return nil, fmt.Errorf("%w: insert with Returning requires One", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("insert"); err != nil {
		return nil, err
	}

	return b.exec.ExecContext(ctx, query, args...)
}

// One executes the insert with Returning and scans the returned row into
// dest, which must be a pointer to the entity's struct type. sql.ErrNoRows is
// preserved when no row comes back. It is rejected without Returning.
func (b *Insert) One(ctx context.Context, dest any) error {
	query, args, err := b.Build()

	if err != nil {
		return err
	}

	if !b.returning {
		return fmt.Errorf("%w: insert.One requires Returning", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("insert"); err != nil {
		return err
	}

	return b.exec.QueryRowContext(ctx, query, args...).Scan(dest)
}
