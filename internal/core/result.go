package core

import (
	"database/sql"
)

// Result exposes driver result information in a driver-agnostic way.
// A false availability flag means the information could not be obtained;
// the underlying result error is not exposed. Zero with true is a valid value.
type Result interface {
	RowsAffected() (int64, bool)
	LastInsertId() (int64, bool)
}

type resultAdapter struct {
	r sql.Result
}

func (ra resultAdapter) RowsAffected() (int64, bool) {
	if ra.r == nil {
		return 0, false
	}
	n, err := ra.r.RowsAffected()

	if err != nil {
		return 0, false
	}

	return n, true
}

func (ra resultAdapter) LastInsertId() (int64, bool) {
	if ra.r == nil {
		return 0, false
	}
	id, err := ra.r.LastInsertId()

	if err != nil {
		return 0, false
	}

	return id, true
}

func wrapResult(r sql.Result) Result {
	if r == nil {
		return resultAdapter{}
	}

	return resultAdapter{r: r}
}
