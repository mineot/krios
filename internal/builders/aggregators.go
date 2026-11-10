package builders

type Aggregate string

const (
	Avg   Aggregate = "AVG"
	Count Aggregate = "COUNT"
	Max   Aggregate = "MAX"
	Min   Aggregate = "MIN"
	Sum   Aggregate = "SUM"
)
