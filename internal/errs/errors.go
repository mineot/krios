// Package errs holds sentinel errors shared by Krios internal packages.
package errs

import "errors"

var (
	ErrInvalidConfig      = errors.New("krios: invalid configuration")
	ErrInvalidQuery       = errors.New("krios: invalid query")
	ErrClosed             = errors.New("krios: database is closed or uninitialized")
	ErrUnsupported        = errors.New("krios: unsupported database version or capability")
	ErrActiveTransactions = errors.New("krios: database has active transactions")
	ErrVersionDetection   = errors.New("krios: database version detection failed")
)
