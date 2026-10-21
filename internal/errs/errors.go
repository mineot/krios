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

	// ErrInvalidQuery indicates that a chained query was built with an
	// invalid construction: an unknown column or entity, a nil operand, a
	// duplicate selection, an empty logical group or any other state that
	// cannot be rendered into SQL. It is produced before execution.
	ErrInvalidQuery = errors.New("krios: invalid query")

	// ErrFileNotFound indicates that an embedded SQL resource could not be
	// loaded: a nil file system, an invalid or missing path, a directory
	// instead of a file, or a blank file. It wraps fs.ErrNotExist when the
	// path does not exist.
	ErrFileNotFound = errors.New("krios: embedded SQL file not found or invalid")

	// ErrPartialMigration indicates that a migration run cannot proceed
	// because a version is incomplete or history holds an in_progress or
	// failed record. Managed objects without history cause ErrInvalidQuery.
	// An incomplete migration is never continued automatically; the consumer
	// must resolve it explicitly. Use
	// errors.As with *PartialMigrationError for the structured context.
	ErrPartialMigration = errors.New("krios: partial migration state blocks execution")
)

// PartialMigrationError carries the context of a blocked migration run. It
// satisfies errors.As and wraps ErrPartialMigration through errors.Is. When it
// wraps an underlying cause, Unwrap preserves it.
type PartialMigrationError struct {
	// Direction is the affected direction: "up" or "down".
	Direction string

	// TargetVersion is the desired version of the blocked run.
	TargetVersion int

	// LastVersion is the last successfully applied version.
	LastVersion int

	// LastFile is the file of the partial record, when known.
	LastFile string

	// Status is the recorded state: "in_progress" or "failed".
	Status string

	// Message describes the block.
	Message string

	cause error
}

// Error renders the message with the structured context.
func (e *PartialMigrationError) Error() string {
	return fmt.Sprintf("%s: %s (direction %s, target %d, last version %d, file %q, status %q)",
		ErrPartialMigration, e.Message, e.Direction, e.TargetVersion, e.LastVersion, e.LastFile, e.Status)
}

// Unwrap exposes ErrPartialMigration and any wrapped cause to errors.Is.
func (e *PartialMigrationError) Unwrap() []error {
	if e.cause != nil {
		return []error{ErrPartialMigration, e.cause}
	}

	return []error{ErrPartialMigration}
}

// NewPartialMigrationError builds a PartialMigrationError. When cause is
// non-nil it is preserved for errors.Is/errors.As.
func NewPartialMigrationError(direction string, targetVersion, lastVersion int, lastFile, status, message string, cause error) error {
	return &PartialMigrationError{
		Direction:     direction,
		TargetVersion: targetVersion,
		LastVersion:   lastVersion,
		LastFile:      lastFile,
		Status:        status,
		Message:       message,
		cause:         cause,
	}
}

// NormalizeTxError maps the native sql.ErrTxDone to ErrTxDone, preserving the
// original cause for errors.Is/errors.As. Other errors pass through unchanged.
func NormalizeTxError(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("%w: %w", ErrTxDone, err)
	}
	return err
}
