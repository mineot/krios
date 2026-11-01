package compiler

// Fail retains the first non-nil error and prevents any further emission.
// A failed compilation requires a new writer; there is no reset operation.
func (w *Writer) Fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// Err reports the first compilation error without exposing partial output.
func (w *Writer) Err() error {
	return w.err
}
