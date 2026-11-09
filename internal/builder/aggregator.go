package builder

type Aggregator string

const (
	Avg   Aggregator = "AVG"
	Count Aggregator = "COUNT"
	Max   Aggregator = "MAX"
	Min   Aggregator = "MIN"
	Sum   Aggregator = "SUM"
)

func (a Aggregator) Build(target string) string {
	return string(a) + "(" + target + ")"
}
