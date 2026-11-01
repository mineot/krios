package connection

import (
	"database/sql"
	"sync"

	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

func (db *DB) reserveTransaction() (*sql.DB, dialect.Info, func(), error) {
	if db == nil {
		return nil, dialect.Info{}, nil, errs.ErrClosed
	}

	db.mu.Lock()

	defer db.mu.Unlock()

	if db.closed || db.pool == nil {
		return nil, dialect.Info{}, nil, errs.ErrClosed
	}

	db.activeTransactions++
	var once sync.Once

	release := func() {
		once.Do(func() {
			db.mu.Lock()
			defer db.mu.Unlock()
			db.activeTransactions--
		})
	}

	return db.pool, db.info, release, nil
}
