package core

import (
	"database/sql"
	"errors"
)

// Row is a wrapper around *sql.Row that preserves sql.ErrNoRows on Scan
// and does not expose the underlying raw pointer to the consumer.
type Row struct {
	r   *sql.Row
	err error
}

// Scan copies the values from the matched row into the provided destinations.
// It returns sql.ErrNoRows if no row was found.
func (rw Row) Scan(dest ...any) error {
	if rw.err != nil {
		return rw.err
	}
	if rw.r == nil {
		return errors.New("krios: row is invalid")
	}

	return normalizeTxError(rw.r.Scan(dest...))
}

func wrapRow(r *sql.Row) Row {
	return Row{r: r}
}

// Rows is an iterator over query results. The caller is responsible for
// calling Close(), including on early returns, and checking Err after iteration.
// Close is idempotent and preserves Err. Use one consumer for Next and Scan;
// consuming the same iterator concurrently is not supported.
type Rows struct {
	r *sql.Rows
}

// Next prepares the next row for reading.
func (rw *Rows) Next() bool {
	if rw == nil || rw.r == nil {
		return false
	}

	return rw.r.Next()
}

// Scan copies the values from the current row into the provided destinations.
func (rw *Rows) Scan(dest ...any) error {
	if rw == nil || rw.r == nil {
		return errors.New("krios: rows are closed or invalid")
	}

	return rw.r.Scan(dest...)
}

// Err returns the error, if any, that was encountered during iteration.
func (rw *Rows) Err() error {
	if rw == nil || rw.r == nil {
		return nil
	}

	return rw.r.Err()
}

// Close closes the underlying rows iterator. It is idempotent.
func (rw *Rows) Close() error {
	if rw == nil || rw.r == nil {
		return nil
	}
	return rw.r.Close()
}

func wrapRows(r *sql.Rows) *Rows {
	if r == nil {
		return nil
	}

	return &Rows{r: r}
}
