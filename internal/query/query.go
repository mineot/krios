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
	fields []entity.Expr
	wheres []entity.Condition
	joins  []join
	groups []entity.Column
	orders []entity.Sort

	distinct  bool
	limit     int
	offset    int
	limitSet  bool
	offsetSet bool
}

type join struct {
	ent  *entity.Entity
	kind JoinKind
	on   entity.Condition
}

type selectionItem struct {
	expr  entity.Expr
	alias string
}

type buildOptions struct {
	paged     bool
	limit     int
	offset    int
	omitOrder bool
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

// Fields appends expressions (columns or aggregates) to the select list, in
// call order. Without Only, the selection contains only these expressions;
// with Only, they follow the selected columns. Aliases are validated when the
// query is rendered.
func (s *Select) Fields(exprs ...entity.Expr) *Select {
	s.fields = append(s.fields, exprs...)
	return s
}

// Distinct removes duplicate result rows.
func (s *Select) Distinct() *Select {
	s.distinct = true
	return s
}

// GroupBy appends grouping columns, in call order. Repeated calls accumulate.
func (s *Select) GroupBy(cols ...entity.Column) *Select {
	s.groups = append(s.groups, cols...)
	return s
}

// OrderBy appends ORDER BY criteria, in call order. Repeated calls accumulate.
// Criteria are built with Asc/Desc on a column or aggregate.
func (s *Select) OrderBy(sorts ...entity.Sort) *Select {
	s.orders = append(s.orders, sorts...)
	return s
}

// Limit sets the maximum number of rows returned. The last call wins;
// Limit(0) returns no rows. Negative values are reported by Build with
// errs.ErrInvalidQuery.
func (s *Select) Limit(n int) *Select {
	s.limit = n
	s.limitSet = true
	return s
}

// Offset sets the number of rows skipped. The last call wins. A positive
// offset requires Limit for portability across the supported databases.
func (s *Select) Offset(n int) *Select {
	s.offset = n
	s.offsetSet = true
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
// (unknown column or entity, nil operand, duplicate entity, selection or
// output alias, invalid join kind, limit or offset) are reported with
// errs.ErrInvalidQuery.
func (s *Select) Build() (string, []any, error) {
	var opts buildOptions

	if s != nil {
		if s.limitSet {
			if s.limit < 0 {
				return "", nil, fmt.Errorf("%w: limit must not be negative", errs.ErrInvalidQuery)
			}
			opts.paged = true
			opts.limit = s.limit
		}

		if s.offsetSet {
			if s.offset < 0 {
				return "", nil, fmt.Errorf("%w: offset must not be negative", errs.ErrInvalidQuery)
			}
			if s.offset > 0 && !s.limitSet {
				return "", nil, fmt.Errorf("%w: offset requires a limit", errs.ErrInvalidQuery)
			}
			opts.offset = s.offset
		}
	}

	return s.build(opts)
}

func (s *Select) build(opts buildOptions) (string, []any, error) {
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

	items, err := s.selection()

	if err != nil {
		return "", nil, err
	}

	if err := validateExprEntities(s.fields, tables); err != nil {
		return "", nil, err
	}

	if err := validateExprEntities(s.groups, tables); err != nil {
		return "", nil, err
	}

	if err := validateExprEntities(s.orders, tables); err != nil {
		return "", nil, err
	}

	var (
		b    strings.Builder
		args []any
	)

	b.WriteString("SELECT ")

	if s.distinct {
		b.WriteString("DISTINCT ")
	}

	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}

		if err := item.render(s.dialect, &b, &args); err != nil {
			return "", nil, err
		}
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

	if len(s.groups) > 0 {
		b.WriteString(" GROUP BY ")

		for i := range s.groups {
			if i > 0 {
				b.WriteString(", ")
			}

			if err := s.groups[i].ExprInto(s.dialect, &b, &args); err != nil {
				return "", nil, err
			}
		}
	}

	if !opts.omitOrder && len(s.orders) > 0 {
		b.WriteString(" ORDER BY ")

		for i := range s.orders {
			if i > 0 {
				b.WriteString(", ")
			}

			if err := s.orders[i].ExprInto(s.dialect, &b, &args); err != nil {
				return "", nil, err
			}
		}
	}

	if opts.paged {
		b.WriteString(" LIMIT ")
		b.WriteString(s.dialect.Placeholder(len(args) + 1))
		args = append(args, opts.limit)

		if opts.offset > 0 {
			b.WriteString(" OFFSET ")
			b.WriteString(s.dialect.Placeholder(len(args) + 1))
			args = append(args, opts.offset)
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

// selection returns the select list: the Only subset when set, followed by the
// explicit Fields expressions; without Only and Fields it contains every
// mapped column of the base entity in declaration order. Output labels are
// validated for uniqueness.
func (s *Select) selection() ([]selectionItem, error) {
	mapped := s.ent.Columns()
	items := make([]selectionItem, 0, len(mapped)+len(s.fields))

	switch {
	case len(s.only) > 0:
		seen := make(map[string]bool, len(s.only))

		for _, name := range s.only {
			if !slices.Contains(mapped, name) {
				return nil, fmt.Errorf("%w: unknown column %q in table %q", errs.ErrInvalidQuery, name, s.ent.Table())
			}

			if seen[name] {
				return nil, fmt.Errorf("%w: duplicate column %q in Only", errs.ErrInvalidQuery, name)
			}

			seen[name] = true
			items = append(items, selectionItem{expr: s.ent.Col(name)})
		}
	case len(s.fields) == 0:
		for _, name := range mapped {
			items = append(items, selectionItem{expr: s.ent.Col(name)})
		}
	}

	for _, f := range s.fields {
		if f == nil {
			return nil, fmt.Errorf("%w: selection contains a nil expression", errs.ErrInvalidQuery)
		}

		alias := strings.TrimSpace(f.Alias())

		if f.Alias() != "" && alias == "" {
			return nil, fmt.Errorf("%w: output alias must not be blank", errs.ErrInvalidQuery)
		}

		items = append(items, selectionItem{expr: f, alias: alias})
	}

	labels := make(map[string]bool, len(items))
	for _, item := range items {
		label := item.label()

		if label == "" {
			continue
		}

		if labels[label] {
			return nil, fmt.Errorf("%w: duplicate output column %q in selection", errs.ErrInvalidQuery, label)
		}

		labels[label] = true
	}
	return items, nil
}

func (i selectionItem) label() string {
	if i.alias != "" {
		return i.alias
	}

	if col, ok := i.expr.(entity.Column); ok {
		return col.Name()
	}

	return ""
}

func (i selectionItem) render(d dialect.Dialect, b *strings.Builder, args *[]any) error {
	if err := i.expr.ExprInto(d, b, args); err != nil {
		return err
	}

	if i.alias != "" {
		b.WriteString(" AS ")
		b.WriteString(d.QuoteIdent(i.alias))
	}

	return nil
}

type exprLike interface {
	Entities() []*entity.Entity
}

func validateExprEntities[T exprLike](items []T, tables map[string]bool) error {
	for _, item := range items {
		for _, ent := range item.Entities() {
			if ent == nil {
				return fmt.Errorf("%w: expression references an empty entity", errs.ErrInvalidQuery)
			}

			if !tables[ent.Table()] {
				return fmt.Errorf("%w: table %q is referenced but not part of the query", errs.ErrInvalidQuery, ent.Table())
			}
		}
	}
	return nil
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
