// Package sqlfile loads consumer-embedded SQL resources and executes them on
// the DB or the transaction that created the runner. The file text is passed
// to the driver unchanged: parameters are supplied separately, using the
// markers the configured driver accepts, and no syntax or placeholder
// translation happens (E07-P01). Execution reuses the Krios executor contract,
// so the DB/Tx guards and the transaction policy of E03 are preserved.
package sqlfile

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
)

// executor is the Krios execution contract satisfied by *DB and *Tx. It is
// declared here structurally so that sqlfile does not depend on the access or
// transaction packages; using it preserves the DB/Tx guards and the
// transaction policy of E03 unchanged.
type executor interface {
	ExecContext(context.Context, string, ...any) (result.Result, error)
	QueryContext(context.Context, string, ...any) (*scan.Rows, error)
	QueryRowContext(context.Context, string, ...any) scan.Row
}

// SQLFile is the immutable reference to a consumer-embedded SQL resource. It
// is created by NewSQLFile, holds the file name and its loaded text and is
// safe for concurrent reuse: it retains no connection, dialect or mutable
// state (E07-P01 §3).
type SQLFile struct {
	name string
	text string
}

// NewSQLFile loads path from fsys and returns the immutable resource. The path
// follows the io/fs semantics expected by the fs package: slash-separated,
// neither ".", "./" prefixed nor absolute. embed.FS satisfies fs.FS.
//
// Loading is eager: the text is read once here. A nil file system, an invalid
// or missing path, a directory or a blank file fail with errs.ErrFileNotFound
// before any database access. fs.ErrNotExist is wrapped when the path does not
// exist.
func NewSQLFile(fsys fs.FS, path string) (*SQLFile, error) {
	if fsys == nil {
		return nil, fmt.Errorf("%w: nil file system for %q", errs.ErrFileNotFound, path)
	}

	if !fs.ValidPath(path) {
		return nil, fmt.Errorf("%w: invalid path %q", errs.ErrFileNotFound, path)
	}

	info, err := fs.Stat(fsys, path)

	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", errs.ErrFileNotFound, path, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("%w: %q is a directory", errs.ErrFileNotFound, path)
	}

	data, err := fs.ReadFile(fsys, path)

	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", errs.ErrFileNotFound, path, err)
	}

	text := string(data)

	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: %q is blank", errs.ErrFileNotFound, path)
	}

	return &SQLFile{name: path, text: text}, nil
}

// Name returns the path the resource was loaded from. It is used in error
// messages and logs.
func (f *SQLFile) Name() string {
	if f == nil {
		return ""
	}

	return f.name
}

// Text returns the loaded SQL text. The returned string is a copy snapshot of
// the resource content; the original value sent to the driver is this text.
func (f *SQLFile) Text() string {
	if f == nil {
		return ""
	}

	return f.text
}

// File is the runner that executes one SQLFile through the DB or Tx that
// created it. It is created by DB.File or Tx.File, is mutable in place (one
// runner per execution; not safe for concurrent reuse) and remembers the first
// construction failure (first-wins), reported before the executor is reached.
type File struct {
	exec executor
	file *SQLFile
	err  error
}

// NewFile creates a runner bound to exec for the resource f. It is used by the
// DB and Tx layers. A missing resource or executor is remembered and reported
// by the execution methods.
func NewFile(exec executor, f *SQLFile) *File {
	b := &File{exec: exec, file: f}

	if f == nil {
		b.remember(fmt.Errorf("%w: file runner requires a SQLFile", errs.ErrInvalidQuery))
	}

	return b
}

// remember stores the first construction failure; later failures are dropped.
func (b *File) remember(err error) {
	if err != nil && b.err == nil {
		b.err = err
	}
}

// prepare returns the loaded text and validated state, or the first
// construction failure. The remembered failure is reported before the executor
// guards apply.
func (b *File) prepare(op string) (string, error) {
	if b == nil || b.file == nil {
		return "", fmt.Errorf("%w: %s requires a SQLFile", errs.ErrInvalidQuery, op)
	}

	if b.err != nil {
		return "", b.err
	}

	if b.exec == nil {
		return "", fmt.Errorf("%w: %s has no executor", errs.ErrInvalidQuery, op)
	}

	return b.file.text, nil
}

// Exec executes the resource without reading rows and returns the execution
// information (krios.Result with RowsAffected and LastInsertId availability).
// Parameters are bound separately and the DB/Tx guards of E03 apply unchanged.
func (b *File) Exec(ctx context.Context, args ...any) (result.Result, error) {
	query, err := b.prepare("file exec")

	if err != nil {
		return nil, err
	}

	return b.exec.ExecContext(ctx, query, args...)
}

// Query executes the resource and returns the iterator. The consumer must
// close it and check Err, or use All. DB/Tx guards and transaction policy
// apply unchanged.
func (b *File) Query(ctx context.Context, args ...any) (*scan.Rows, error) {
	query, err := b.prepare("file query")

	if err != nil {
		return nil, err
	}

	return b.exec.QueryContext(ctx, query, args...)
}

// QueryRow executes the resource expected to match at most one row. Scan
// consumes the row and preserves sql.ErrNoRows. Construction failures are
// deferred to Scan.
func (b *File) QueryRow(ctx context.Context, args ...any) scan.Row {
	query, err := b.prepare("file query row")

	if err != nil {
		return scan.ErrorRow(err)
	}

	return b.exec.QueryRowContext(ctx, query, args...)
}

// One executes the resource and scans the single expected row into dest, which
// must be a pointer to a struct type (mapped as Row.Scan). It returns
// sql.ErrNoRows when no row matches. Parameters are bound separately.
func (b *File) One(ctx context.Context, dest any, args ...any) error {
	return b.QueryRow(ctx, args...).Scan(dest)
}

// All executes the resource and maps every result row into dest, which must be
// a pointer to a slice of structs (mapped as Rows.All: columns matched to
// fields by name). The iterator is closed on every path. Parameters are bound
// separately.
func (b *File) All(ctx context.Context, dest any, args ...any) error {
	rows, err := b.Query(ctx, args...)

	if err != nil {
		return err
	}

	return rows.All(dest)
}
