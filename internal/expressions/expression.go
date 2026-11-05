package expressions

// Expression is an immutable aggregate expression belonging to one client.
type Expression struct {
	owner    *Identity
	function string
	operand  any
	err      error
}

func aggregate(owner *Identity, function string, operand any) *Expression {
	e := &Expression{owner: owner, function: function, operand: operand}

	if !ValidIdentity(owner) {
		e.err = invalid("invalid operand origin")
		return e
	}

	switch value := operand.(type) {
	case string:
		e.operand = NewReference(owner, value)
		e.err = validateReference(e.operand.(*Reference), owner)

	case *Reference:
		e.err = validateReference(value, owner)

	case *Expression:
		e.err = validateExpression(value, owner)

	default:
		e.err = invalid("unsupported aggregate operand")
	}

	return e
}

func validateExpression(e *Expression, owner *Identity) error {
	if e == nil || !ValidIdentity(e.owner) || !ValidIdentity(owner) {
		return invalid("invalid expression")
	}

	if e.err != nil {
		return e.err
	}

	if e.owner != owner {
		return invalid("operand belongs to another client")
	}

	return nil
}

// ReadExpression exposes only immutable operands, without adding public methods
// to the aliased type. Construction is restricted to the aggregate functions.
func ReadExpression(e *Expression, owner *Identity) (string, any, error) {
	if err := validateExpression(e, owner); err != nil {
		return "", nil, err
	}

	return e.function, e.operand, nil
}
