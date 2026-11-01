package expression

import (
	"strings"

	"github.com/mineot/krios/internal/compiler"
)

// WriteExpr validates and renders a projection or operand expression.
func WriteExpr(w *compiler.Writer, expr Expr, use ExprUse) {
	if w.Err() != nil {
		return
	}

	if err := ValidateExpr(expr, use); err != nil {
		w.Fail(err)
		return
	}

	writeExpr(w, expr)
}

// writeExpr renders an expression already validated in its enclosing fragment.
func writeExpr(w *compiler.Writer, expr Expr) {
	switch expr.kind {
	case columnExpr:
		writeColumn(w, expr.name)
	case valueExpr:
		w.Parameter(expr.value)
	case aggregateExpr:
		switch expr.aggregate {
		case countAggregate:
			w.Syntax("COUNT(")
		case avgAggregate:
			w.Syntax("AVG(")
		case sumAggregate:
			w.Syntax("SUM(")
		case minAggregate:
			w.Syntax("MIN(")
		case maxAggregate:
			w.Syntax("MAX(")
		default:
			w.Fail(invalid("invalid aggregate"))
			return
		}
		writeColumn(w, expr.name)
		w.Syntax(")")
	default:
		w.Fail(invalid("uninitialized or invalid expression"))
		return
	}

	if expr.alias.set {
		w.Syntax(" AS ")
		w.Identifier(expr.alias.name)
	}
}

// writeColumn receives validated qualification and wildcard placement.
func writeColumn(w *compiler.Writer, name string) {
	for i, part := range strings.Split(name, ".") {
		if i > 0 {
			w.Syntax(".")
		}

		if part == "*" {
			w.Syntax("*")
		} else {
			w.Identifier(part)
		}
	}
}
