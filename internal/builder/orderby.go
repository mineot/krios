package builder

type Direction string

const (
	AscDirection  Direction = "ASC"
	DescDirection Direction = "DESC"
)

type OrderBy struct {
	Name        string
	Orientation string
	Table       string
}
