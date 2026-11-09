package builder

import (
	"errors"

	"github.com/mineot/krios/internal/dialects"
)

type Column struct {
	Aggregator Aggregator
	As         string
	Name       string
	Table      string
	err        error
}

func (c *Column) GetError() error {
	return c.err
}

func (c *Column) getName(d dialects.Dialect) string {
	return d.Quotes(c.Name)
}

func (c *Column) hasName() bool {
	return c.Name != ""
}

func (c *Column) getAs(d dialects.Dialect) string {
	return d.Quotes(c.As)
}

func (c *Column) hasAs() bool {
	return c.As != ""
}

func (c *Column) getTable(d dialects.Dialect) string {
	return d.Quotes(c.Table)
}

func (c *Column) hasTable() bool {
	return c.Table != ""
}

func (c *Column) getAggregator(target string) string {
	return c.Aggregator.Build(target)
}

func (c *Column) hasAggregator() bool {
	return string(c.Aggregator) != ""
}

func (c *Column) Build(d dialects.Dialect) string {
	if c.hasName() == false {
		c.err = errors.New("Krios: column name is required!")
		return ""
	}

	column := c.getName(d)

	if c.hasTable() {
		column = c.getTable(d) + "." + column
	}

	if c.hasAggregator() {
		column = c.getAggregator(column)
	}

	if c.hasAs() {
		column = column + " AS " + c.getAs(d)
	}

	return column
}
