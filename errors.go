package krios

import "github.com/mineot/krios/internal/errs"

var (
	// ErrInvalidConfig identifies invalid initialization arguments.
	ErrInvalidConfig = errs.ErrInvalidConfig
	// ErrInvalidQuery identifies invalid query arguments, including nil contexts.
	ErrInvalidQuery = errs.ErrInvalidQuery
	// ErrClosed identifies an instance that is closed or uninitialized.
	ErrClosed = errs.ErrClosed
	// ErrUnsupported identifies an unsupported version or capability.
	ErrUnsupported = errs.ErrUnsupported
	// ErrActiveTransactions indicates that transactions must finish before Close.
	ErrActiveTransactions = errs.ErrActiveTransactions
	// ErrVersionDetection identifies failure to query or interpret the version.
	ErrVersionDetection = errs.ErrVersionDetection
)
