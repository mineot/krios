package dialects

import "fmt"

func (d *Dialect) Placeholder(index uint) string {
	if *d == Postgres {
		return fmt.Sprintf("$%d", index)
	}

	return "?"
}
