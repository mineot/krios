// Package transaction coordinates native transactions and their registration.
package transaction

import (
	"context"
	"database/sql"
	"sync"

	"github.com/mineot/krios/internal/dialect"
)

// Tx owns transaction lifecycle state. It must not be copied after use.
type Tx struct {
	mu      sync.Mutex
	native  *sql.Tx
	ctx     context.Context
	info    dialect.Info
	done    bool
	stop    func() bool
	release func()
}

// New wraps a successfully opened native transaction and its release callback.
// All arguments must be initialized by the owning connection.
func New(ctx context.Context, native *sql.Tx, info dialect.Info, release func()) *Tx {
	tx := &Tx{native: native, ctx: ctx, info: info, release: release}
	tx.watchContext()
	return tx
}

// finish requires mu and runs only after native finalization has been handled.
func (tx *Tx) finish() {
	tx.done = true

	if tx.stop != nil {
		tx.stop()
		tx.stop = nil
	}

	if tx.release != nil {
		tx.release()
		tx.release = nil
	}
}
