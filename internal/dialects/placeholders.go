package dialects

import "fmt"

func (d *Dialect) Placeholder(index int) string {
	if *d == Postgres {
		return fmt.Sprintf("$%d", index)
	}

	return "?"
}
