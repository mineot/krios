package core

import (
	"database/sql"
	"errors"
)

// Row is a wrapper around *sql.Row that preserves sql.ErrNoRows on Scan
// and does not expose the underlying raw pointer to the consumer. When the
// row was produced by a Tx, any Scan failure other than sql.ErrNoRows is
// recorded on that transaction and blocks Commit.
type Row struct {
	r   *sql.Row
	tx  *Tx
	err error
}

// Scan copies the values from the matched row into the provided destinations.
// It returns sql.ErrNoRows if no row was found. A single struct pointer is
// filled from the mapped entity columns in declaration order (see EntityOf);
// any other destination is scanned positionally. Any failure is recorded on
// the owning transaction and blocks Commit (see Tx.Commit).
func (rw Row) Scan(dest ...any) error {
	if rw.err != nil {
		rw.record(rw.err)
		return rw.err
	}

	if rw.r == nil {
		err := errors.New("krios: row is invalid")
		rw.record(err)
		return err
	}

	if len(dest) == 1 {
		ent, mapped, err := entityDest(dest[0])

		if err != nil {
			rw.record(err)
			return err
		}

		if mapped {
			return rw.scanEntity(ent, dest[0])
		}
	}

	err := rw.r.Scan(dest...)
	rw.record(err)

	return normalizeTxError(err)
}

func (rw Row) record(err error) {
	if rw.tx != nil {
		rw.tx.recordLate(err)
	}
}

func wrapRow(r *sql.Row) Row {
	return Row{r: r}
}

// Rows is an iterator over query results. The caller is responsible for
// calling Close(), including on early returns, and checking Err after iteration.
// Close is idempotent and preserves Err. Use one consumer for Next and Scan;
// consuming the same iterator concurrently is not supported. When the rows
// were produced by a Tx, any scan or iteration failure is recorded on that
// transaction and blocks Commit.
type Rows struct {
	r  *sql.Rows
	tx *Tx
}

// Next prepares the next row for reading. When iteration ends with a failure,
// the failure is recorded on the owning transaction and blocks Commit.
func (rw *Rows) Next() bool {
	if rw == nil || rw.r == nil {
		return false
	}

	if rw.r.Next() {
		return true
	}

	rw.record(rw.r.Err())
	return false
}

// Scan copies the values from the current row into the provided destinations.
// A failure is recorded on the owning transaction and blocks Commit.
func (rw *Rows) Scan(dest ...any) error {
	if rw == nil || rw.r == nil {
		return errors.New("krios: rows are closed or invalid")
	}

	err := rw.r.Scan(dest...)
	rw.record(err)
	return err
}

// Err returns the error, if any, that was encountered during iteration.
// A non-nil iteration failure is recorded on the owning transaction and
// blocks Commit.
func (rw *Rows) Err() error {
	if rw == nil || rw.r == nil {
		return nil
	}

	err := rw.r.Err()
	rw.record(err)
	return err
}

// Columns returns the result set column names in driver order. A failure is
// recorded on the owning transaction and blocks Commit.
func (rw *Rows) Columns() ([]string, error) {
	if rw == nil || rw.r == nil {
		return nil, errors.New("krios: rows are closed or invalid")
	}

	columns, err := rw.r.Columns()

	if err != nil {
		rw.record(err)
		return nil, err
	}

	return columns, nil
}

// Close closes the underlying rows iterator. It is idempotent. Failures
// surfaced by closing or by pending iteration are recorded on the owning
// transaction and block Commit.
func (rw *Rows) Close() error {
	if rw == nil || rw.r == nil {
		return nil
	}

	err := rw.r.Close()
	rw.record(err)
	rw.record(rw.r.Err())
	return err
}

func (rw *Rows) record(err error) {
	if rw == nil || rw.tx == nil || err == nil {
		return
	}

	rw.tx.recordLate(err)
}

func wrapRows(r *sql.Rows) *Rows {
	if r == nil {
		return nil
	}

	return &Rows{r: r}
}
