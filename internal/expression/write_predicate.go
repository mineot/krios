package expression

import "github.com/mineot/krios/internal/compiler"

// WritePredicate validates the entire tree before emitting any part of it.
// Explicit groups and parameter order are preserved, even around constants.
func WritePredicate(w *compiler.Writer, predicate Predicate) {
	if w.Err() != nil {
		return
	}

	if err := ValidatePredicate(predicate); err != nil {
		w.Fail(err)
		return
	}

	writePredicate(w, predicate)
}

// writePredicate visits only nodes validated by WritePredicate.
func writePredicate(w *compiler.Writer, predicate Predicate) {
	node := predicate.node

	switch node.kind {
	case andPredicate, wherePredicate, onPredicate, orPredicate:
		w.Syntax("(")
		for i, child := range node.children {
			if i > 0 {
				if node.kind == orPredicate {
					w.Syntax(" OR ")
				} else {
					w.Syntax(" AND ")
				}
			}
			writePredicate(w, child)
		}
		w.Syntax(")")
	case eqPredicate:
		writeBinary(w, node, " = ")
	case nePredicate:
		writeBinary(w, node, " <> ")
	case ltPredicate:
		writeBinary(w, node, " < ")
	case ltePredicate:
		writeBinary(w, node, " <= ")
	case gtPredicate:
		writeBinary(w, node, " > ")
	case gtePredicate:
		writeBinary(w, node, " >= ")
	case likePredicate:
		writeBinary(w, node, " LIKE ")
	case notLikePredicate:
		writeBinary(w, node, " NOT LIKE ")
	case betweenPredicate, notBetweenPredicate:
		writeExpr(w, node.operands[0])
		if node.kind == betweenPredicate {
			w.Syntax(" BETWEEN ")
		} else {
			w.Syntax(" NOT BETWEEN ")
		}
		writeExpr(w, node.operands[1])
		w.Syntax(" AND ")
		writeExpr(w, node.operands[2])
	case isNullPredicate, isNotNullPredicate:
		writeExpr(w, node.operands[0])
		if node.kind == isNullPredicate {
			w.Syntax(" IS NULL")
		} else {
			w.Syntax(" IS NOT NULL")
		}
	case inPredicate, notInPredicate:
		writeMembership(w, node)
	default:
		w.Fail(invalid("invalid predicate kind"))
	}
}

func writeBinary(w *compiler.Writer, node *predicateNode, operator string) {
	writeExpr(w, node.operands[0])
	w.Syntax(operator)
	writeExpr(w, node.operands[1])
}

func writeMembership(w *compiler.Writer, node *predicateNode) {
	if len(node.values) == 0 {
		if node.kind == inPredicate {
			w.Syntax("1 = 0")
		} else {
			w.Syntax("1 = 1")
		}

		return
	}

	writeExpr(w, node.operands[0])

	if node.kind == inPredicate {
		w.Syntax(" IN (")
	} else {
		w.Syntax(" NOT IN (")
	}

	for i, value := range node.values {
		if i > 0 {
			w.Syntax(", ")
		}

		w.Parameter(value)
	}

	w.Syntax(")")
}
