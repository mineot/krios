package transaction

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/mineot/krios/internal/errs"
)

// readSQL validates before reading. Callers must revalidate after success by
// delegating to Query/Exec; read errors take precedence over later cancellation.
func (tx *Tx) readSQL(ctx context.Context, files fs.FS, path string) (string, error) {
	if _, err := tx.command(ctx); err != nil {
		return "", err
	}

	if files == nil {
		return "", fmt.Errorf("%w: filesystem must not be nil", errs.ErrInvalidQuery)
	}

	if !fs.ValidPath(path) || path == "." {
		return "", fmt.Errorf("%w: invalid SQL file path", errs.ErrInvalidQuery)
	}

	content, err := fs.ReadFile(files, path)

	if err != nil {
		return "", fmt.Errorf("krios: read SQL file: %w", err)
	}

	return string(content), nil
}
