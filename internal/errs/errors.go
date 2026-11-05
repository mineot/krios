package errs

import "errors"

var (
	ErrInvalidConfig    = errors.New("krios: invalid configuration")
	ErrInvalidQuery     = errors.New("krios: invalid query")
	ErrInvalidMigration = errors.New("krios: invalid migration")
)
