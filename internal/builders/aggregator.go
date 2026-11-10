package builders

type Aggregate string

const (
	Count Aggregate = "COUNT"
	Max   Aggregate = "MAX"
	Min   Aggregate = "MIN"
	Sum   Aggregate = "SUM"
	Avg   Aggregate = "AVG"
)

func (a Aggregate) String() string {
	return string(a)
}
