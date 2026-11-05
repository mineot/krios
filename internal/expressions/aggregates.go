package expressions

func Count(owner *Identity, operand any) *Expression {
	return aggregate(owner, "COUNT", operand)
}

func Max(owner *Identity, operand any) *Expression {
	return aggregate(owner, "MAX", operand)
}

func Min(owner *Identity, operand any) *Expression {
	return aggregate(owner, "MIN", operand)
}

func Avg(owner *Identity, operand any) *Expression {
	return aggregate(owner, "AVG", operand)
}

func Sum(owner *Identity, operand any) *Expression {
	return aggregate(owner, "SUM", operand)
}
