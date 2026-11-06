package selects

type Select struct {
	columns   []string
	from      string
	joins     []string
	criterias []string
	orders    []string
	groups    []string
	paginate  []string
}
