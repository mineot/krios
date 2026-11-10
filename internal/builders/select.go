package builders

type Select struct {
	Columns  []string
	Tables   []Table
	Joins    []string
	Where    []string
	Having   []string
	GroupBy  []string
	OrderBy  []string
	Paginate []string
}
