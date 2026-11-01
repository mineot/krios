package expression

// Value constructs a parameter without conversion, copying or evaluation.
// Referenced values must remain unchanged until query execution finishes.
func Value(value any) Expr {
	return Expr{kind: valueExpr, value: value}
}
