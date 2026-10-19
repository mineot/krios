package entity

import (
	"fmt"
	"strings"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

// likeEscape is the escape character used in generated LIKE patterns and in
// the ESCAPE clause. A neutral character avoids the "'\'" string literal that
// would be invalid in MySQL under default escaping rules.
const likeEscape = '!'

// Column references a mapped column of an entity. It is produced by
// (*Entity).Col and carries the comparators that build query conditions.
// Column values are rendered as qualified identifiers; any other operand
// becomes a bound parameter.
type Column struct {
	entity *Entity
	name   string
}

// Col returns a reference to the named column of e. The name is validated
// when the condition is rendered or executed; unknown columns fail with
// errs.ErrInvalidQuery before reaching the driver. A nil entity is reported
// the same way.
func (e *Entity) Col(name string) Column {
	return Column{entity: e, name: name}
}

// Condition is an opaque boolean predicate used by Where and Join. Build
// conditions with the comparators on Column and with And/Or. The zero
// Condition is invalid and fails rendering with errs.ErrInvalidQuery.
type Condition struct {
	node node
}

// LikeMode selects how a value given to Column.Like is surrounded with the
// SQL wildcard character.
type LikeMode int

const (
	// Contains matches values that contain the given text (%value%).
	Contains LikeMode = iota
	// StartsWith matches values that begin with the given text (value%).
	StartsWith
	// EndsWith matches values that end with the given text (%value).
	EndsWith
)

// node is an internal AST node rendered to SQL through the dialect. Rendering
// collects bound parameters into args, in textual order.
type node interface {
	render(d dialect.Dialect, w *strings.Builder, args *[]any) error
}

// Render renders the condition fragment and its bound parameters for the
// given dialect. Construction failures (unknown column, nil operand, empty
// group) surface here with errs.ErrInvalidQuery, before any execution.
func (c Condition) Render(d dialect.Dialect) (string, []any, error) {
	var args []any

	fragment, err := c.RenderInto(d, &args)

	if err != nil {
		return "", nil, err
	}

	return fragment, args, nil
}

// RenderInto renders the condition fragment like Render, but appends the
// bound parameters to args instead of a fresh slice. Placeholder numbering
// continues from the current length of args, so several conditions rendered
// into the same slice (join ON clauses and multiple WHERE conditions) share
// one sequence in textual order.
func (c Condition) RenderInto(d dialect.Dialect, args *[]any) (string, error) {
	if d == nil {
		return "", fmt.Errorf("%w: condition requires a dialect", errs.ErrInvalidQuery)
	}

	var b strings.Builder

	if err := c.nodeOrErr().render(d, &b, args); err != nil {
		return "", err
	}

	return b.String(), nil
}

// Entities returns the distinct entities referenced by the condition's
// columns, for the query builder to validate that every referenced entity
// participates in the query.
func (c Condition) Entities() []*Entity {
	set := map[*Entity]struct{}{}
	collectEntities(c.nodeOrErr(), set)
	entities := make([]*Entity, 0, len(set))

	for ent := range set {
		entities = append(entities, ent)
	}

	return entities
}

func (c Condition) nodeOrErr() node {
	if c.node == nil {
		return errNode{fmt.Errorf("%w: nil condition", errs.ErrInvalidQuery)}
	}

	return c.node
}

// Eq builds a "=" condition between the column and v. A Column operand is
// rendered as a comparison between columns; any other non-nil value becomes a
// parameter. nil is rejected: use IsNull or IsNotNull.
func (c Column) Eq(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: "=", rhs: operand(v)}}
}

// Ne builds a "<>" condition. See Eq for operand handling.
func (c Column) Ne(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: "<>", rhs: operand(v)}}
}

// Lt builds a "<" condition. See Eq for operand handling.
func (c Column) Lt(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: "<", rhs: operand(v)}}
}

// Lte builds a "<=" condition. See Eq for operand handling.
func (c Column) Lte(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: "<=", rhs: operand(v)}}
}

// Gt builds a ">" condition. See Eq for operand handling.
func (c Column) Gt(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: ">", rhs: operand(v)}}
}

// Gte builds a ">=" condition. See Eq for operand handling.
func (c Column) Gte(v any) Condition {
	return Condition{compareNode{lhs: c.ref(), op: ">=", rhs: operand(v)}}
}

// IsNull builds an "IS NULL" condition.
func (c Column) IsNull() Condition {
	return Condition{nullNode{col: c.ref()}}
}

// IsNotNull builds an "IS NOT NULL" condition.
func (c Column) IsNotNull() Condition {
	return Condition{nullNode{col: c.ref(), neg: true}}
}

// In builds an "IN" condition with one parameter per value. An empty list
// renders the constant FALSE; a nil value inside the list is rejected.
func (c Column) In(values ...any) Condition {
	operands := make([]node, len(values))

	for i, v := range values {
		operands[i] = operand(v)
	}

	return Condition{inNode{col: c.ref(), values: operands}}
}

// Like builds a "LIKE" condition using the given mode. The value is matched
// literally: the escape character, "%" and "_" inside it are escaped and an
// ESCAPE clause is always emitted. Case sensitivity follows the database
// collation and is not adapted.
func (c Column) Like(mode LikeMode, value string) Condition {
	var rhs node
	pattern, err := mode.pattern(value)

	if err != nil {
		rhs = errNode{err}
	} else {
		rhs = paramNode{value: pattern}
	}

	return Condition{likeNode{col: c.ref(), pattern: rhs}}
}

// And combines conditions into a conjunction. An empty group or a nil
// condition inside it fails rendering with errs.ErrInvalidQuery.
func And(conditions ...Condition) Condition {
	return group("AND", conditions)
}

// Or combines conditions into a disjunction, nestable with And and Or. An
// empty group or a nil condition inside it fails rendering with
// errs.ErrInvalidQuery.
func Or(conditions ...Condition) Condition {
	return group("OR", conditions)
}

func group(op string, conditions []Condition) Condition {
	if len(conditions) == 0 {
		return Condition{errNode{fmt.Errorf("%w: %s group requires at least one condition", errs.ErrInvalidQuery, op)}}
	}

	nodes := make([]node, len(conditions))

	for i, c := range conditions {
		nodes[i] = c.nodeOrErr()
	}

	return Condition{logicalNode{op: op, nodes: nodes}}
}

func (c Column) ref() node {
	return columnNode{entity: c.entity, name: c.name}
}

func operand(v any) node {
	switch value := v.(type) {
	case nil:
		return errNode{fmt.Errorf("%w: nil operand; use IsNull or IsNotNull", errs.ErrInvalidQuery)}
	case Column:
		if value.entity == nil {
			return errNode{fmt.Errorf("%w: nil entity in column reference", errs.ErrInvalidQuery)}
		}
		return value.ref()
	default:
		return paramNode{value: value}
	}
}

func (m LikeMode) pattern(value string) (string, error) {
	escaped := escapeLike(value)
	switch m {
	case Contains:
		return "%" + escaped + "%", nil
	case StartsWith:
		return escaped + "%", nil
	case EndsWith:
		return "%" + escaped, nil
	default:
		return "", fmt.Errorf("%w: unknown LIKE mode %d", errs.ErrInvalidQuery, m)
	}
}

func escapeLike(value string) string {
	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		if r == likeEscape || r == '%' || r == '_' {
			b.WriteByte(likeEscape)
		}

		b.WriteRune(r)
	}

	return b.String()
}

// columnNode renders a qualified column reference after validating that the
// entity declares the column.
type columnNode struct {
	entity *Entity
	name   string
}

func (n columnNode) render(d dialect.Dialect, w *strings.Builder, _ *[]any) error {
	if n.entity == nil {
		return fmt.Errorf("%w: nil entity in column reference", errs.ErrInvalidQuery)
	}

	if _, ok := n.entity.byColumn[n.name]; !ok {
		return fmt.Errorf("%w: unknown column %q in table %q", errs.ErrInvalidQuery, n.name, n.entity.table)
	}

	w.WriteString(d.QuoteIdent(n.entity.table))
	w.WriteByte('.')
	w.WriteString(d.QuoteIdent(n.name))

	return nil
}

// paramNode renders a bound parameter placeholder.
type paramNode struct {
	value any
}

func (n paramNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	*args = append(*args, n.value)
	w.WriteString(d.Placeholder(len(*args)))
	return nil
}

// errNode fails rendering with its stored error.
type errNode struct {
	err error
}

func (n errNode) render(dialect.Dialect, *strings.Builder, *[]any) error {
	return n.err
}

// compareNode renders "lhs op rhs".
type compareNode struct {
	lhs node
	op  string
	rhs node
}

func (n compareNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if err := n.lhs.render(d, w, args); err != nil {
		return err
	}

	w.WriteByte(' ')
	w.WriteString(n.op)
	w.WriteByte(' ')

	return n.rhs.render(d, w, args)
}

// logicalNode renders "(a AND b ...)" or "(a OR b ...)".
type logicalNode struct {
	op    string
	nodes []node
}

func (n logicalNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	w.WriteByte('(')

	for i, child := range n.nodes {
		if i > 0 {
			w.WriteByte(' ')
			w.WriteString(n.op)
			w.WriteByte(' ')
		}
		if err := child.render(d, w, args); err != nil {
			return err
		}
	}

	w.WriteByte(')')
	return nil
}

// likeNode renders "col LIKE pattern ESCAPE '!'".
type likeNode struct {
	col     node
	pattern node
}

func (n likeNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if err := n.col.render(d, w, args); err != nil {
		return err
	}

	w.WriteString(" LIKE ")

	if err := n.pattern.render(d, w, args); err != nil {
		return err
	}

	w.WriteString(" ESCAPE '" + string(likeEscape) + "'")
	return nil
}

// inNode renders "col IN (v1, v2, ...)" or the constant FALSE when empty.
type inNode struct {
	col    node
	values []node
}

func (n inNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if len(n.values) == 0 {
		w.WriteString("FALSE")
		return nil
	}

	if err := n.col.render(d, w, args); err != nil {
		return err
	}

	w.WriteString(" IN (")

	for i, value := range n.values {
		if i > 0 {
			w.WriteString(", ")
		}

		if err := value.render(d, w, args); err != nil {
			return err
		}
	}

	w.WriteByte(')')
	return nil
}

// nullNode renders "col IS NULL" or "col IS NOT NULL".
type nullNode struct {
	col node
	neg bool
}

func (n nullNode) render(d dialect.Dialect, w *strings.Builder, args *[]any) error {
	if err := n.col.render(d, w, args); err != nil {
		return err
	}

	if n.neg {
		w.WriteString(" IS NOT NULL")
	} else {
		w.WriteString(" IS NULL")
	}

	return nil
}

func collectEntities(n node, set map[*Entity]struct{}) {
	switch node := n.(type) {
	case columnNode:
		if node.entity != nil {
			set[node.entity] = struct{}{}
		}
	case logicalNode:
		for _, child := range node.nodes {
			collectEntities(child, set)
		}
	case compareNode:
		collectEntities(node.lhs, set)
		collectEntities(node.rhs, set)
	case likeNode:
		collectEntities(node.col, set)
		collectEntities(node.pattern, set)
	case inNode:
		collectEntities(node.col, set)
		for _, value := range node.values {
			collectEntities(value, set)
		}
	case nullNode:
		collectEntities(node.col, set)
	}
}
