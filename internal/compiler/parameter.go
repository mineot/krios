package compiler

import "github.com/mineot/krios/internal/dialect"

// Parameter appends one occurrence without conversion, evaluation or deduplication.
func (w *Writer) Parameter(value any) {
	if w.err != nil {
		return
	}

	placeholder, err := dialect.Placeholder(w.info.Dialect, len(w.args)+1)

	if err != nil {
		w.Fail(err)
		return
	}

	w.args = append(w.args, value)
	w.sql.WriteString(placeholder)
}
