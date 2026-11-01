package krios

import "github.com/mineot/krios/internal/expression"

// Expr is an opaque column, aggregate or parameter expression. Its zero value
// is invalid. Structural validation is deferred to future query compilation.
type Expr = expression.Expr

// Predicate is an opaque condition or explicit group. Its zero value is invalid.
type Predicate = expression.Predicate

// TableRef is an immutable table reference with optional schema and alias.
// Its zero value is invalid.
type TableRef = expression.TableRef

// OrderTerm is an immutable ordering column, direction and NULL placement.
// Its zero value is invalid.
type OrderTerm = expression.OrderTerm
