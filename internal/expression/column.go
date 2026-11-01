package expression

// Column constructs a possibly qualified identifier, never a SQL fragment.
func Column(name string) Expr {
	return Expr{kind: columnExpr, name: name}
}
