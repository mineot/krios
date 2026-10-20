package query

import (
	"context"
	"fmt"

	"github.com/mineot/krios/internal/errs"
)

// Paginate describes one page of a query executed with Select.Paginate.
type Paginate struct {
	// Total is the number of rows matching the query, ignoring ordering,
	// limit and offset.
	Total int64
	// Limit and Offset are the values used for the data query.
	Limit  int
	Offset int
	// Page is the 1-based page number (offset/limit + 1).
	Page int
	// Pages is the total number of pages, zero when Total is zero.
	Pages int
	// HasNext reports whether rows exist after this page.
	HasNext bool
	// HasPrev reports whether rows exist before this page.
	HasPrev bool
}

// Count returns the number of rows the query matches, ignoring ORDER BY, LIMIT
// and OFFSET (including values set on the builder). It counts through
// SELECT COUNT(*) over the query as a derived table, so DISTINCT and GROUP BY
// are preserved. The total is exact at the cost of materializing the inner
// query.
func (s *Select) Count(ctx context.Context) (int64, error) {
	query, args, err := s.countQuery()
	if err != nil {
		return 0, err
	}

	if s.exec == nil {
		return 0, fmt.Errorf("%w: select has no executor", errs.ErrInvalidQuery)
	}

	var total int64

	if err := s.exec.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

// countQuery renders the counting statement independently of the executor so
// the generated SQL and arguments can be validated for every dialect.
func (s *Select) countQuery() (string, []any, error) {
	if s == nil || s.ent == nil {
		return "", nil, fmt.Errorf("%w: select requires a base entity", errs.ErrInvalidQuery)
	}

	if s.dialect == nil {
		return "", nil, fmt.Errorf("%w: select requires a dialect", errs.ErrInvalidQuery)
	}

	inner, args, err := s.build(buildOptions{omitOrder: true})

	if err != nil {
		return "", nil, err
	}

	return "SELECT COUNT(*) FROM (" + inner + ") AS " + s.dialect.QuoteIdent("krios_total"), args, nil
}

// Paginate counts the query, executes the page starting at offset and maps its
// rows into dest (see Rows.All), returning totals and navigation statistics.
// Limit must be greater than zero and offset must not be negative; on error no
// partial result is returned. Paginate does not mutate the builder: any limit
// or offset set on it is ignored only for this call.
//
// On *DB the count and the page are independent queries, so concurrent writes
// can make them diverge; use a transaction for a consistent snapshot. On *Tx
// both queries share the transaction.
func (s *Select) Paginate(ctx context.Context, limit, offset int, dest any) (Paginate, error) {
	if limit <= 0 {
		return Paginate{}, fmt.Errorf("%w: paginate limit must be greater than zero", errs.ErrInvalidQuery)
	}

	if offset < 0 {
		return Paginate{}, fmt.Errorf("%w: paginate offset must not be negative", errs.ErrInvalidQuery)
	}

	total, err := s.Count(ctx)

	if err != nil {
		return Paginate{}, err
	}

	query, args, err := s.build(buildOptions{paged: true, limit: limit, offset: offset})

	if err != nil {
		return Paginate{}, err
	}

	if s.exec == nil {
		return Paginate{}, fmt.Errorf("%w: select has no executor", errs.ErrInvalidQuery)
	}

	rows, err := s.exec.QueryContext(ctx, query, args...)

	if err != nil {
		return Paginate{}, err
	}

	if err := rows.All(dest); err != nil {
		return Paginate{}, err
	}

	pages := int((total + int64(limit) - 1) / int64(limit))

	return Paginate{
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Page:    offset/limit + 1,
		Pages:   pages,
		HasNext: int64(offset)+int64(limit) < total,
		HasPrev: offset > 0,
	}, nil
}
