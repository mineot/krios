package transaction

import "context"

func (tx *Tx) watchContext() {
	// A canceled context may run the callback immediately. Publish stop while
	// holding mu so finalization cannot run before registration is complete.
	tx.mu.Lock()

	defer tx.mu.Unlock()

	tx.stop = context.AfterFunc(tx.ctx, func() {
		// database/sql also watches ctx. ErrTxDone means native finalization
		// has already started; driver cleanup may still be in progress.
		_ = tx.Rollback()
	})
}
