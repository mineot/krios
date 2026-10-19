// Package errs holds the sentinel errors shared by the Krios packages and the
// normalization of native transaction errors.
package errs

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig indicates an invalid configuration: missing required
	// fields, inconsistent values or a negative Timeout. The message details
	// the offending field.
	ErrInvalidConfig = errors.New("krios: invalid configuration")

	// ErrDriverNotLoaded indicates that the driver named in Config.Driver is
	// not registered in database/sql: the consumer did not import the driver
	// or registered it under a different name.
	ErrDriverNotLoaded = errors.New("krios: database driver not loaded for the configured type")

	// ErrConnectionFailed indicates a failure to connect (network, file,
	// permission, database refusal). It wraps the original driver error.
	ErrConnectionFailed = errors.New("krios: failed to connect to database")

	// ErrClosed indicates an operation attempted after Close.
	ErrClosed = errors.New("krios: database access is closed")

	// ErrTxDone indicates an operation attempted on a terminal transaction,
	// including after a native finalization failure or automatic rollback.
	ErrTxDone = errors.New("krios: transaction already committed or rolled back")

	// ErrInvalidEntity indicates that a struct could not be mapped into an
	// entity: it is not a struct, has no mappable fields, uses unsupported
	// fields or tags, or declares an invalid TableName. It also covers invalid
	// destinations passed to Entity.Validate, ValidateAll and Format.
	ErrInvalidEntity = errors.New("krios: invalid entity")

	// ErrValidation indicates that a field rejected its own value through the
	// Validator interface. The error contextualizes the table and column and
	// wraps the cause.
	ErrValidation = errors.New("krios: field validation failed")

	// ErrFormat indicates that a field failed to produce its presentation text
	// through the Formatter interface. The error contextualizes the table and
	// column and wraps the cause.
	ErrFormat = errors.New("krios: field formatting failed")
)

// NormalizeTxError maps the native sql.ErrTxDone to ErrTxDone, preserving the
// original cause for errors.Is/errors.As. Other errors pass through unchanged.
func NormalizeTxError(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("%w: %w", ErrTxDone, err)
	}
	return err
}
