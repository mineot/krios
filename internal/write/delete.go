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

// Delete is a chained DELETE builder for one base entity. It is created by
// DB.Delete or Tx.Delete, is mutable in place (one builder per statement; not
// safe for concurrent reuse) and executes through the DB or Tx that created
// it. At least one Where condition is required (E06-P01 §5); a statement can
// affect several rows.
type Delete struct {
	builder
	wheres []entity.Condition
}

// NewDelete creates a builder bound to exec and rendering with d for the base
// entity ent.
func NewDelete(exec executor, d dialect.Dialect, ent *entity.Entity) *Delete {
	return &Delete{builder: builder{exec: exec, dialect: d, ent: ent}}
}

// Where adds filter conditions, combined with AND. Repeated calls accumulate
// with AND. At least one condition is required before execution
// (E06-P01 §5); there is no shortcut to delete every row.
func (b *Delete) Where(conditions ...entity.Condition) *Delete {
	b.wheres = append(b.wheres, conditions...)
	return b
}

// Returning requests the deleted rows back through the RETURNING clause.
// Without it, Exec returns the execution result; with it, All fills the
// destination. The dialect must support RETURNING (SQLite and PostgreSQL;
// MySQL fails on Build before executing — E06-P01 §7).
func (b *Delete) Returning() *Delete {
	b.returning = true
	return b
}

// Build renders the statement and its bound parameters without executing it.
// Construction failures (missing entity, missing Where, conditions on other
// tables, unsupported RETURNING) are reported with errs.ErrInvalidQuery.
func (b *Delete) Build() (string, []any, error) {
	if b == nil {
		return "", nil, fmt.Errorf("%w: delete requires an entity", errs.ErrInvalidQuery)
	}

	if _, err := b.prepare("delete"); err != nil {
		return "", nil, err
	}

	var (
		w    strings.Builder
		args []any
	)

	w.WriteString("DELETE FROM ")
	w.WriteString(b.dialect.QuoteIdent(b.ent.Table()))

	if err := renderWhere(b.wheres, "delete", &b.builder, &w, &args); err != nil {
		return "", nil, err
	}

	b.renderReturning(&w)

	return w.String(), args, nil
}

// Exec executes the delete and returns the execution information
// (krios.Result with RowsAffected). It is rejected after Returning, where All
// must be used instead. Construction failures and DB/Tx guards are applied
// before execution.
func (b *Delete) Exec(ctx context.Context) (result.Result, error) {
	query, args, err := b.Build()

	if err != nil {
		return nil, err
	}

	if b.returning {
		return nil, fmt.Errorf("%w: delete with Returning requires All", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("delete"); err != nil {
		return nil, err
	}

	return b.exec.ExecContext(ctx, query, args...)
}

// All executes the delete with Returning and maps every returned row into
// dest, which must be a pointer to a slice of the entity's struct type (see
// Rows.All). The iterator is closed on every path. It is rejected without
// Returning.
func (b *Delete) All(ctx context.Context, dest any) error {
	query, args, err := b.Build()

	if err != nil {
		return err
	}

	if !b.returning {
		return fmt.Errorf("%w: delete.All requires Returning", errs.ErrInvalidQuery)
	}

	if err := b.noExecutor("delete"); err != nil {
		return err
	}

	rows, err := b.exec.QueryContext(ctx, query, args...)

	if err != nil {
		return err
	}

	return rows.All(dest)
}
