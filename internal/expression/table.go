package expression

// Table constructs an unqualified table reference; use InSchema for qualification.
func Table(name string) TableRef {
	return TableRef{name: name}
}

// As returns a new reference with the last supplied alias.
func (table TableRef) As(alias string) TableRef {
	table.alias = optionalName{name: alias, set: true}
	return table
}

// InSchema returns a new reference with the last supplied schema.
func (table TableRef) InSchema(schema string) TableRef {
	table.schema = optionalName{name: schema, set: true}
	return table
}
