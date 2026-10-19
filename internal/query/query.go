// Package query builds chained SELECT statements from entity descriptors,
// renders them through the configured dialect and executes them on the DB or
// the transaction that created the builder. Construction is pure; rendering
// (Build) and execution surface every construction failure before reaching
// the driver.
package query

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
// declared here structurally so that query does not depend on the access or
// transaction packages; using it preserves the DB/Tx guards and the
// transaction policy of E03.
type executor interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
	QueryContext(context.Context, string, ...any) (*scan.Rows, error)
	QueryRowContext(context.Context, string, ...any) scan.Row
}

// JoinKind selects the join operation of a Join call.
type JoinKind int

const (
	// InnerJoin keeps only rows with a match on both sides.
	InnerJoin JoinKind = iota
	// LeftJoin keeps every row of the base table, filling joined columns
	// with NULL when there is no match.
	LeftJoin
)

// Select is a chained SELECT builder for one base entity. It is created by
// DB.Select or Tx.Select, is mutable in place (one builder per query; not
// safe for concurrent reuse) and executes through the DB or Tx that created
// it.
type Select struct {
	exec    executor
	dialect dialect.Dialect
	ent     *entity.Entity

	only   []string
	wheres []entity.Condition
	joins  []join
}

type join struct {
	ent  *entity.Entity
	kind JoinKind
	on   entity.Condition
}

// New creates a builder bound to exec and rendering with d for the base
// entity ent.
func New(exec executor, d dialect.Dialect, ent *entity.Entity) *Select {
	return &Select{exec: exec, dialect: d, ent: ent}
}

// Only restricts the selected columns to the named ones, validated against
// the base entity when rendered. Repeated calls append; an empty selection
// keeps the default (every mapped column, in field declaration order).
func (s *Select) Only(names ...string) *Select {
	s.only = append(s.only, names...)
	return s
}

// Where adds filter conditions, combined with AND. Repeated calls accumulate
// with AND; no conditions means no WHERE clause.
func (s *Select) Where(conditions ...entity.Condition) *Select {
	s.wheres = append(s.wheres, conditions...)
	return s
}

// Join appends a join of ent to the query with the given kind and ON
// condition. The joined entity must differ from the base entity and from the
// entities of previously joined tables. The declaration order of joins is
// preserved.
func (s *Select) Join(ent *entity.Entity, kind JoinKind, on entity.Condition) *Select {
	s.joins = append(s.joins, join{ent: ent, kind: kind, on: on})
	return s
}

// Build renders the query and its bound parameters without executing it. The
// parameters are in textual order and follow the SQL. Construction failures
// (unknown column or entity, nil operand, duplicate entity or selection,
// invalid join kind) are reported with errs.ErrInvalidQuery.
func (s *Select) Build() (string, []any, error) {
	if s == nil || s.ent == nil {
		return "", nil, fmt.Errorf("%w: select requires a base entity", errs.ErrInvalidQuery)
	}
	if s.dialect == nil {
		return "", nil, fmt.Errorf("%w: select requires a dialect", errs.ErrInvalidQuery)
	}

	tables := map[string]bool{s.ent.Table(): true}
	for _, j := range s.joins {
		if j.ent == nil {
			return "", nil, fmt.Errorf("%w: join requires an entity", errs.ErrInvalidQuery)
		}
		if tables[j.ent.Table()] {
			return "", nil, fmt.Errorf("%w: table %q appears more than once", errs.ErrInvalidQuery, j.ent.Table())
		}
		tables[j.ent.Table()] = true
	}

	columns, err := s.selection()
	if err != nil {
		return "", nil, err
	}

	var (
		b    strings.Builder
		args []any
	)

	b.WriteString("SELECT ")
	for i, name := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(qualify(s.dialect, s.ent.Table(), name))
	}
	b.WriteString(" FROM ")
	b.WriteString(s.dialect.QuoteIdent(s.ent.Table()))

	for _, j := range s.joins {
		kind, err := joinKeyword(j.kind)
		if err != nil {
			return "", nil, err
		}
		b.WriteString(" ")
		b.WriteString(kind)
		b.WriteString(" ")
		b.WriteString(s.dialect.QuoteIdent(j.ent.Table()))
		b.WriteString(" ON ")

		if err := renderCondition(j.on, s.dialect, tables, &b, &args); err != nil {
			return "", nil, err
		}
	}

	if len(s.wheres) > 0 {
		b.WriteString(" WHERE ")
		for i, cond := range s.wheres {
			if i > 0 {
				b.WriteString(" AND ")
			}
			if err := renderCondition(cond, s.dialect, tables, &b, &args); err != nil {
				return "", nil, err
			}
		}
	}

	return b.String(), args, nil
}

// Rows executes the query and returns the iterator. The consumer must close
// it and check Err, or use All. Construction failures are returned before the
// executor is reached; DB/Tx guards and transaction policy apply unchanged.
func (s *Select) Rows(ctx context.Context) (*scan.Rows, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	if s.exec == nil {
		return nil, fmt.Errorf("%w: select has no executor", errs.ErrInvalidQuery)
	}
	return s.exec.QueryContext(ctx, query, args...)
}

// Row executes the query expected to match at most one row. Scan consumes
// the row and preserves sql.ErrNoRows. Construction failures are deferred to
// Scan.
func (s *Select) Row(ctx context.Context) scan.Row {
	query, args, err := s.Build()
	if err != nil {
		return scan.ErrorRow(err)
	}
	if s.exec == nil {
		return scan.ErrorRow(fmt.Errorf("%w: select has no executor", errs.ErrInvalidQuery))
	}
	return s.exec.QueryRowContext(ctx, query, args...)
}

// All executes the query and maps every result row into dest, which must be a
// pointer to a slice of structs (see Rows.All). The iterator is closed on
// every path and the slice is replaced only on success.
func (s *Select) All(ctx context.Context, dest any) error {
	rows, err := s.Rows(ctx)
	if err != nil {
		return err
	}
	return rows.All(dest)
}

// One executes the query and scans the single expected row into dest. It
// returns sql.ErrNoRows when no row matches. A struct destination is filled
// positionally, so the select list must follow the declaration order of the
// mapped fields (see Row.Scan).
func (s *Select) One(ctx context.Context, dest any) error {
	return s.Row(ctx).Scan(dest)
}

// selection returns the columns to select: the Only subset when set,
// otherwise every mapped column of the base entity in declaration order.
func (s *Select) selection() ([]string, error) {
	mapped := s.ent.Columns()
	if len(s.only) == 0 {
		return mapped, nil
	}

	seen := make(map[string]bool, len(s.only))
	columns := make([]string, 0, len(s.only))
	for _, name := range s.only {
		if !slices.Contains(mapped, name) {
			return nil, fmt.Errorf("%w: unknown column %q in table %q", errs.ErrInvalidQuery, name, s.ent.Table())
		}
		if seen[name] {
			return nil, fmt.Errorf("%w: duplicate column %q in Only", errs.ErrInvalidQuery, name)
		}
		seen[name] = true
		columns = append(columns, name)
	}
	return columns, nil
}

func renderCondition(cond entity.Condition, d dialect.Dialect, tables map[string]bool, b *strings.Builder, args *[]any) error {
	for _, ent := range cond.Entities() {
		if !tables[ent.Table()] {
			return fmt.Errorf("%w: table %q is referenced but not part of the query", errs.ErrInvalidQuery, ent.Table())
		}
	}

	fragment, err := cond.RenderInto(d, args)
	if err != nil {
		return err
	}
	b.WriteString(fragment)
	return nil
}

func qualify(d dialect.Dialect, table, column string) string {
	return d.QuoteIdent(table) + "." + d.QuoteIdent(column)
}

func joinKeyword(kind JoinKind) (string, error) {
	switch kind {
	case InnerJoin:
		return "INNER JOIN", nil
	case LeftJoin:
		return "LEFT JOIN", nil
	default:
		return "", fmt.Errorf("%w: unknown join kind %d", errs.ErrInvalidQuery, kind)
	}
}
