// Package expression constructs and validates immutable SQL expression trees.
// It does not compile SQL, evaluate parameter values or execute queries.
package expression

// Expr is an opaque scalar or projection expression. Its zero value is invalid.
type Expr struct {
	kind      exprKind
	name      string
	value     any
	aggregate aggregateKind
	alias     optionalName
}

// As returns an expression with the supplied projection alias.
// The last alias wins; an explicitly empty alias is invalid at validation time.
func (expr Expr) As(alias string) Expr {
	expr.alias = optionalName{name: alias, set: true}
	return expr
}

// Predicate holds an immutable condition or group. Its zero value is invalid.
type Predicate struct {
	node *predicateNode
}

// TableRef identifies a table, optionally qualified by schema and alias.
type TableRef struct {
	name   string
	schema optionalName
	alias  optionalName
}

// OrderTerm identifies an ordering column, direction and NULL placement.
type OrderTerm struct {
	column    string
	direction direction
	nulls     nullPlacement
}

type optionalName struct {
	name string
	set  bool
}

type exprKind uint8

const (
	columnExpr exprKind = iota + 1
	valueExpr
	aggregateExpr
)

type aggregateKind uint8

const (
	countAggregate aggregateKind = iota + 1
	avgAggregate
	sumAggregate
	minAggregate
	maxAggregate
)

type predicateKind uint8

const (
	eqPredicate predicateKind = iota + 1
	nePredicate
	ltPredicate
	ltePredicate
	gtPredicate
	gtePredicate
	inPredicate
	notInPredicate
	likePredicate
	notLikePredicate
	betweenPredicate
	notBetweenPredicate
	isNullPredicate
	isNotNullPredicate
	andPredicate
	orPredicate
	wherePredicate
	onPredicate
)

type predicateNode struct {
	kind     predicateKind
	operands []Expr
	values   []any
	children []Predicate
}

type direction uint8

const (
	ascending direction = iota + 1
	descending
)

type nullPlacement uint8

const (
	nullsDefault nullPlacement = iota
	nullsFirst
	nullsLast
)
