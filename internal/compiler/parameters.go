package compiler

// Parameter preserves the value as supplied; conversion belongs to database/sql
// and the driver. Even slices represent one value here.
func (c *Compiler) Parameter(value any) {
	if c.err != nil {
		return
	}

	c.args = append(c.args, value)
	c.Write(c.dialect.Placeholder(len(c.args)))
}
