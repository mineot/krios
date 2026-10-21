// Package scan provides the deferred single-row and iterator results produced
// by Krios queries, together with reflection-based entity mapping. It records
// late read failures through a recorder injected by the owner (a transaction);
// scan itself does not depend on the transaction package.
package scan

import (
	"database/sql"
	"errors"

	"github.com/mineot/krios/internal/errs"
)

// Row is a wrapper around *sql.Row that preserves sql.ErrNoRows on Scan
// and does not expose the underlying raw pointer to the consumer. When a
// recorder was bound, any Scan failure other than sql.ErrNoRows is reported to
// the owner (a transaction) and blocks Commit.
type Row struct {
	r   *sql.Row
	rec func(error)
	err error
}

// Scan copies the values from the matched row into the provided destinations.
// It returns sql.ErrNoRows if no row was found. A single struct pointer is
// filled from the mapped entity columns in declaration order (see
// entity.EntityOf); any other destination is scanned positionally. Any failure
// is reported to the owner and blocks Commit (see transaction.Tx.Commit).
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
			rw.releaseRow()

			return err
		}

		if mapped {
			return rw.scanEntity(ent, dest[0])
		}
	}

	err := rw.r.Scan(dest...)
	rw.record(err)

	return errs.NormalizeTxError(err)
}

// releaseRow concludes the native row lifecycle when Scan rejects the
// destination before consuming it. database/sql only releases the underlying
// result set through (*sql.Row).Scan, so a rejected destination that skips it
// would leak the native rows. The discarded error only reports the destination
// arity mismatch; the caller's cause is the rejected destination itself.
func (rw Row) releaseRow() {
	_ = rw.r.Scan()
}

func (rw Row) record(err error) {
	if rw.rec != nil {
		rw.rec(err)
	}
}

// WrapRow adapts a native *sql.Row to Row. The recorder is attached later with
// BindRow.
func WrapRow(r *sql.Row) Row {
	return Row{r: r}
}

// ErrorRow returns a Row that defers err to Scan without a native row. It is
// used when the operation was rejected before reaching the driver, such as a
// canceled context.
func ErrorRow(err error) Row {
	return Row{err: err}
}

// BindRow attaches the recorder that receives late read failures observed on
// rw. A nil recorder leaves rw without an owner to report to.
func BindRow(rw *Row, rec func(error)) {
	rw.rec = rec
}

// NativeRowError returns the error reported by the underlying *sql.Row, or nil
// when there is no native row. It lets an owner observe a native terminal
// transaction error without exposing the raw pointer.
func NativeRowError(rw Row) error {
	if rw.r == nil {
		return nil
	}

	return rw.r.Err()
}

// Rows is an iterator over query results. The caller is responsible for
// calling Close(), including on early returns, and checking Err after iteration.
// Close is idempotent and preserves Err. Use one consumer for Next and Scan;
// consuming the same iterator concurrently is not supported. When a recorder
// was bound, any scan or iteration failure is reported to the owner (a
// transaction) and blocks Commit.
type Rows struct {
	r   *sql.Rows
	rec func(error)
}

// Next prepares the next row for reading. When iteration ends with a failure,
// the failure is reported to the owner and blocks Commit.
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
// A failure is reported to the owner and blocks Commit.
func (rw *Rows) Scan(dest ...any) error {
	if rw == nil || rw.r == nil {
		return errors.New("krios: rows are closed or invalid")
	}

	err := rw.r.Scan(dest...)
	rw.record(err)
	return err
}

// Err returns the error, if any, that was encountered during iteration.
// A non-nil iteration failure is reported to the owner and blocks Commit.
func (rw *Rows) Err() error {
	if rw == nil || rw.r == nil {
		return nil
	}

	err := rw.r.Err()
	rw.record(err)
	return err
}

// Columns returns the result set column names in driver order. A failure is
// reported to the owner and blocks Commit.
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
// surfaced by closing or by pending iteration are reported to the owner and
// block Commit.
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
	if rw == nil || rw.rec == nil || err == nil {
		return
	}

	rw.rec(err)
}

// WrapRows adapts a native *sql.Rows to Rows. It returns nil for a nil input.
// The recorder is attached later with BindRows.
func WrapRows(r *sql.Rows) *Rows {
	if r == nil {
		return nil
	}

	return &Rows{r: r}
}

// BindRows attaches the recorder that receives late read failures observed on
// rw. It is a no-op on a nil iterator.
func BindRows(rw *Rows, rec func(error)) {
	if rw != nil {
		rw.rec = rec
	}
}
