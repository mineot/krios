// Package compiler accumulates SQL fragments and positional arguments.
package compiler

import (
	"slices"
	"strings"

	"github.com/mineot/krios/internal/dialect"
)

// Writer belongs to one compilation and must be obtained through New.
// It must not be copied or shared between goroutines.
type Writer struct {
	info dialect.Info
	sql  strings.Builder
	args []any
	err  error
}

// New starts an independent compilation with detected dialect metadata.
func New(info dialect.Info) (*Writer, error) {
	if err := dialect.ValidateInfo(info); err != nil {
		return nil, err
	}

	return &Writer{info: info}, nil
}

// Info returns a copy of the initialization snapshot, including capabilities.
func (w *Writer) Info() dialect.Info {
	return w.info
}

// Syntax appends library-controlled syntax only. Application identifiers and
// values must use Identifier and Parameter instead.
func (w *Writer) Syntax(token string) {
	if w.err != nil {
		return
	}

	w.sql.WriteString(token)
}

// Identifier appends one validated component, escaped for the active dialect.
func (w *Writer) Identifier(name string) {
	if w.err != nil {
		return
	}

	quoted, err := dialect.QuoteIdentifier(w.info.Dialect, name)

	if err != nil {
		w.Fail(err)
		return
	}

	w.sql.WriteString(quoted)
}

// Result returns SQL and an independent argument slice, or no partial result
// on failure. Argument payloads retain their original references.
func (w *Writer) Result() (string, []any, error) {
	if w.err != nil {
		return "", nil, w.err
	}

	return w.sql.String(), slices.Clone(w.args), nil
}
