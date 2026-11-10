package builders

type Column struct {
	Aggregate Aggregate
	As        string
	Config    Config
	Name      string
	Table     string

	err error
}
