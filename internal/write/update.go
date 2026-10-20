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

// Update is a chained UPDATE builder for one base entity. It is created by
// DB.Update or Tx.Update, is mutable in place (one builder per statement; not
// safe for concurrent reuse) and executes through the DB or Tx that created
// it. At least one Where condition and one column to write are required
// (E06-P01 §4–§5); a statement can affect several rows.
type Update struct {
	builder
	values []entity.WriteValue
	wheres []entity.Condition
}

// NewUpdate creates a builder bound to exec and rendering with d for the base
// entity ent.
func NewUpdate(exec executor, d dialect.Dialect, ent *entity.Entity) *Update {
	return &Update{builder: builder{exec: exec, dialect: d, ent: ent}}
}

// SetValues assigns the values to write. record must be T or *T of the
// entity's type. The values are extracted and the field validators run at
// call time (E06-P01 §8); failures are remembered and reported by Build and
// by the execution methods before the statement reaches the driver. A later
// call replaces the values of an earlier successful call; the first failure
// wins. The values are a snapshot: later changes to record do not affect the
// statement.
//
// The write policy of E06-P01 §4.2 applies: nil pointer fields are not
// updated, sql.Null* with Valid false writes NULL, and non-nullable fields
// are written even with zero values — omit them with Omit when needed.
func (b *Update) SetValues(record any) *Update {
	if b == nil {
		return b
	}

	if b.ent == nil {
		b.remember(fmt.Errorf("%w: update requires an entity", errs.ErrInvalidQuery))
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

// Where adds filter conditions, combined with AND. Repeated calls accumulate
// with AND. At least one condition is required before execution
// (E06-P01 §5); there is no shortcut to update every row.
func (b *Update) Where(conditions ...entity.Condition) *Update {
	b.wheres = append(b.wheres, conditions...)
	return b
}

// Omit excludes the named columns from the assignment, leaving them
// unchanged. Unknown or duplicated names are reported by Build with
// errs.ErrInvalidQuery. Repeated calls accumulate.
func (b *Update) Omit(names ...string) *Update {
	b.omit = append(b.omit, names...)
	return b
}

// Returning requests the updated rows back through the RETURNING clause.
// Without it, Exec returns the execution result; with it, All fills the
// destination. The dialect must support RETURNING (SQLite and PostgreSQL;
// MySQL fails on Build before executing — E06-P01 §7).
func (b *Update) Returning() *Update {
	b.returning = true
	return b
}

// Build renders the statement and its bound parameters without executing it,
// in textual order: assignments first, then WHERE conditions, so placeholder
// numbering (including PostgreSQL $n) follows the SQL text. Construction
// failures (missing entity or SetValues, no column to write after the value
// policy and Omit, missing Where, conditions on other tables, invalid Omit,
// unsupported RETURNING, remembered SetValues failure) are reported with
// errs.ErrInvalidQuery, errs.ErrInvalidEntity or errs.ErrValidation.
func (b *Update) Build() (string, []any, error) {
	if b == nil {
		return "", nil, fmt.Errorf("%w: update requires an entity", errs.ErrInvalidQuery)
	}

	omit, err := b.prepare("update")

	if err != nil {
		return "", nil, err
	}

	if b.values == nil {
		return "", nil, fmt.Errorf("%w: update requires SetValues", errs.ErrInvalidQuery)
	}

	sets := make([]entity.WriteValue, 0, len(b.values))

	for _, value := range b.values {
		if value.Absent || omit[value.Column] {
			continue
		}

		sets = append(sets, value)
	}

	if len(sets) == 0 {
		return "", nil, fmt.Errorf("%w: update has no columns to write", errs.ErrInvalidQuery)
	}

	var (
		w    strings.Builder
		args []any
	)

	w.WriteString("UPDATE ")
	w.WriteString(b.dialect.QuoteIdent(b.ent.Table()))
	w.WriteString(" SET ")

	for i, col := range sets {
		if i > 0 {
			w.WriteString(", ")
		}

		w.WriteString(b.dialect.QuoteIdent(col.Column))
		w.WriteString(" = ")
		args = append(args, col.Value)
		w.WriteString(b.dialect.Placeholder(len(args)))
	}

	if err := renderWhere(b.wheres, "update", &b.builder, &w, &args); err != nil {
		return "", nil, err
	}

	b.renderReturning(&w)

	return w.String(), args, nil
}

// Exec executes the update and returns the execution information
// (krios.Result with RowsAffected). It is rejected after Returning, where All
// must be used instead. Construction failures and DB/Tx guards are applied
// before execution.
func (b *Update) Exec(ctx context.Context) (result.Result, error) {
	query, args, err := b.Build()

	if err != nil {
		return nil, err
	}

	if b.returning {
		return nil, fmt.Errorf("%w: update with Returning requires All", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("update"); err != nil {
		return nil, err
	}

	return b.exec.ExecContext(ctx, query, args...)
}

// All executes the update with Returning and maps every returned row into
// dest, which must be a pointer to a slice of the entity's struct type (see
// Rows.All). The iterator is closed on every path. It is rejected without
// Returning.
func (b *Update) All(ctx context.Context, dest any) error {
	query, args, err := b.Build()

	if err != nil {
		return err
	}

	if !b.returning {
		return fmt.Errorf("%w: update.All requires Returning", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("update"); err != nil {
		return err
	}

	rows, err := b.exec.QueryContext(ctx, query, args...)

	if err != nil {
		return err
	}

	return rows.All(dest)
}
