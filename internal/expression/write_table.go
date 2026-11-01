package expression

import "github.com/mineot/krios/internal/compiler"

// WriteTable validates and renders a table reference, with schema and alias.
func WriteTable(w *compiler.Writer, table TableRef) {
	if w.Err() != nil {
		return
	}

	if err := ValidateTable(table); err != nil {
		w.Fail(err)
		return
	}

	if table.schema.set {
		w.Identifier(table.schema.name)
		w.Syntax(".")
	}

	w.Identifier(table.name)

	if table.alias.set {
		w.Syntax(" AS ")
		w.Identifier(table.alias.name)
	}
}
