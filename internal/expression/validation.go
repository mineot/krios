package expression

import "fmt"

// ExprUse selects structural rules only; dialect and builder rules come later.
type ExprUse uint8

const (
	Projection ExprUse = iota + 1
	Operand
)

// ValidateTable checks a table and its explicitly configured schema and alias.
func ValidateTable(table TableRef) error {
	if err := validateComponent(table.name); err != nil {
		return fmt.Errorf("table name: %w", err)
	}

	if table.schema.set {
		if err := validateComponent(table.schema.name); err != nil {
			return fmt.Errorf("table schema: %w", err)
		}
	}

	if table.alias.set {
		if err := validateComponent(table.alias.name); err != nil {
			return fmt.Errorf("table alias: %w", err)
		}
	}

	return nil
}

// ValidateExpr checks an expression for projection or operand use.
// Parameter payloads are never inspected, converted or evaluated.
func ValidateExpr(expr Expr, use ExprUse) error {
	if use != Projection && use != Operand {
		return invalid("invalid expression use")
	}

	var wildcard bool

	switch expr.kind {
	case columnExpr:
		var err error
		wildcard, err = validateColumn(expr.name, use == Projection)
		if err != nil {
			return err
		}
	case valueExpr:
		// Value(nil) is valid; conversion belongs to database/sql.
	case aggregateExpr:
		switch expr.aggregate {
		case countAggregate, avgAggregate, sumAggregate, minAggregate, maxAggregate:
		default:
			return invalid("invalid aggregate")
		}
		if expr.aggregate != countAggregate || expr.name != "*" {
			if _, err := validateColumn(expr.name, false); err != nil {
				return fmt.Errorf("aggregate column: %w", err)
			}
		}
	default:
		return invalid("uninitialized or invalid expression")
	}

	if expr.alias.set {
		if use != Projection || wildcard {
			return invalid("alias is not allowed in this expression position")
		}

		if err := validateComponent(expr.alias.name); err != nil {
			return fmt.Errorf("expression alias: %w", err)
		}
	}

	return nil
}

// ValidateOrder checks structure without deciding dialect support for NULL order.
func ValidateOrder(order OrderTerm) error {
	if _, err := validateColumn(order.column, false); err != nil {
		return fmt.Errorf("ordering column: %w", err)
	}

	if order.direction != ascending && order.direction != descending {
		return invalid("invalid ordering direction")
	}

	if order.nulls != nullsDefault && order.nulls != nullsFirst && order.nulls != nullsLast {
		return invalid("invalid NULL ordering")
	}

	return nil
}

// ValidatePredicate walks operands and children in order, returning the first
// structural error even when another child determines the group's truth value.
func ValidatePredicate(predicate Predicate) error {
	node := predicate.node

	if node == nil {
		return invalid("uninitialized predicate")
	}

	var operandCount int
	switch node.kind {
	case andPredicate, orPredicate, wherePredicate, onPredicate:
		if len(node.children) == 0 {
			return invalid("empty predicate group")
		}

		for i, child := range node.children {
			if err := ValidatePredicate(child); err != nil {
				return fmt.Errorf("predicate child %d: %w", i, err)
			}
		}
		return nil
	case eqPredicate, nePredicate, ltPredicate, ltePredicate, gtPredicate, gtePredicate,
		likePredicate, notLikePredicate:
		operandCount = 2
	case betweenPredicate, notBetweenPredicate:
		operandCount = 3
	case inPredicate, notInPredicate, isNullPredicate, isNotNullPredicate:
		operandCount = 1
	default:
		return invalid("invalid predicate kind")
	}

	if len(node.operands) != operandCount {
		return invalid("invalid predicate operand count")
	}

	for i, operand := range node.operands {
		if err := ValidateExpr(operand, Operand); err != nil {
			return fmt.Errorf("predicate operand %d: %w", i, err)
		}
	}

	return nil
}
