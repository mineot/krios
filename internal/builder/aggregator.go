package builder

type Aggregator string

const (
	AvgAggregator   Aggregator = "AVG"
	CountAggregator Aggregator = "COUNT"
	MaxAggregator   Aggregator = "MAX"
	MinAggregator   Aggregator = "MIN"
	SumAggregator   Aggregator = "SUM"
)

func (a Aggregator) Build(target string) string {
	return string(a) + "(" + target + ")"
}
