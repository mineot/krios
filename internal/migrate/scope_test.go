package migrate

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
	"github.com/mineot/krios/internal/sqlfile"
)

// This file covers C03-P02: the transactional classification of a migration
// file must be explicit for every dialect (D10: allowlist + denylist,
// "in doubt, false"), reject transaction control statements before anything
// is applied (D11), scan SQLite/PostgreSQL files instead of trusting the
// dialect flag blindly (D12) and treat quoted tokens conservatively (D13).

func mdFor(t *testing.T, typ config.DBType) dialect.MigrationDialect {
	t.Helper()

	md, ok := dialect.For(typ).(dialect.MigrationDialect)

	if !ok {
		t.Fatalf("%s has no migration capabilities", typ)
	}

	return md
}

func TestFileTransactionalByDialect(t *testing.T) {
	ddl := "CREATE TABLE users (id INT PRIMARY KEY);"
	dml := "INSERT INTO users VALUES (1);\nUPDATE users SET name = 'a';"
	mixed := "INSERT INTO users VALUES (1);\nCREATE TABLE audit (id INT);"
	commentOnly := "-- schema note\n/* nothing to run */"
	control := "INSERT INTO users VALUES (1);\nCOMMIT;"
	unterminated := "INSERT INTO users VALUES (1); /* oops"

	cases := []struct {
		name  string
		typ   config.DBType
		text  string
		want  bool
		isErr bool
	}{
		{"sqlite ddl", config.DBTypeSQLite, ddl, true, false},
		{"postgres ddl", config.DBTypePostgres, ddl, true, false},
		{"mysql ddl", config.DBTypeMySQL, ddl, false, false},
		{"mysql dml only", config.DBTypeMySQL, dml, true, false},
		{"sqlite dml only", config.DBTypeSQLite, dml, true, false},
		{"mysql mixed", config.DBTypeMySQL, mixed, false, false},
		{"sqlite comment only", config.DBTypeSQLite, commentOnly, true, false},
		{"mysql comment only", config.DBTypeMySQL, commentOnly, true, false},
		{"sqlite control rejected", config.DBTypeSQLite, control, false, true},
		{"postgres control rejected", config.DBTypePostgres, control, false, true},
		{"mysql control rejected", config.DBTypeMySQL, control, false, true},
		{"sqlite unterminated comment", config.DBTypeSQLite, unterminated, false, true},
		{"postgres unterminated comment", config.DBTypePostgres, unterminated, false, true},
		{"mysql unterminated comment", config.DBTypeMySQL, unterminated, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fileTransactional(tc.text, mdFor(t, tc.typ))

			if tc.isErr {
				if err == nil {
					t.Fatalf("fileTransactional(%q) error = nil, want error", tc.text)
				}
				return
			}

			if err != nil {
				t.Fatalf("fileTransactional(%q) error = %v", tc.text, err)
			}

			if got != tc.want {
				t.Errorf("fileTransactional(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"single", "INSERT INTO t VALUES (1);", []string{"INSERT INTO t VALUES (1)"}},
		{"multiple", "INSERT INTO t VALUES (1);\nCREATE TABLE u (id INT);", []string{
			"INSERT INTO t VALUES (1)",
			"CREATE TABLE u (id INT)",
		}},
		{"semicolon in string", "INSERT INTO t VALUES ('a;b');", []string{"INSERT INTO t VALUES ('a;b')"}},
		{"semicolon in comment", "-- keep; me\nINSERT INTO t VALUES (1);", []string{
			"-- keep; me\nINSERT INTO t VALUES (1)",
		}},
		{"empty fragments dropped", ";;\nINSERT INTO t VALUES (1);;", []string{"INSERT INTO t VALUES (1)"}},
		{"no trailing semicolon", "UPDATE t SET a = 1", []string{"UPDATE t SET a = 1"}},
		{"quoted identifier keeps semicolon", "INSERT INTO `we;ird` VALUES (1);", []string{
			"INSERT INTO `we;ird` VALUES (1)",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitStatements(tc.text)

			if len(got) != len(tc.want) {
				t.Fatalf("splitStatements(%q) = %q, want %q", tc.text, got, tc.want)
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitStatements(%q)[%d] = %q, want %q", tc.text, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestLoadStepTransactionalScope(t *testing.T) {
	fsys := fstest.MapFS{
		"m/001_users.sql": &fstest.MapFile{Data: []byte("CREATE TABLE users (id INT PRIMARY KEY);")},
		"m/002_seed.sql":  &fstest.MapFile{Data: []byte("INSERT INTO users VALUES (1);")},
		"m/003_bad.sql":   &fstest.MapFile{Data: []byte("INSERT INTO users VALUES (2);\nCOMMIT;")},
	}

	cfg := &config.MigrationsConfig{FS: fsys}

	cases := []struct {
		name string
		typ  config.DBType
		file string
		want bool
	}{
		{"sqlite ddl", config.DBTypeSQLite, "m/001_users.sql", true},
		{"postgres ddl", config.DBTypePostgres, "m/001_users.sql", true},
		{"mysql ddl", config.DBTypeMySQL, "m/001_users.sql", false},
		{"mysql dml", config.DBTypeMySQL, "m/002_seed.sql", true},
		{"postgres dml", config.DBTypePostgres, "m/002_seed.sql", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step, err := loadStep(cfg, 1, DirectionUp, tc.file, mdFor(t, tc.typ))

			if err != nil {
				t.Fatalf("loadStep(%q) = %v", tc.file, err)
			}

			if step.Transactional != tc.want {
				t.Errorf("loadStep(%q).Transactional = %v, want %v", tc.file, step.Transactional, tc.want)
			}
		})
	}

	for _, typ := range []config.DBType{config.DBTypeSQLite, config.DBTypePostgres, config.DBTypeMySQL} {
		if _, err := loadStep(cfg, 3, DirectionUp, "m/003_bad.sql", mdFor(t, typ)); err == nil {
			t.Errorf("%s loadStep(control file) error = nil, want error", typ)
		} else if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("%s loadStep(control file) = %v, want ErrInvalidQuery", typ, err)
		} else if !strings.Contains(err.Error(), "COMMIT") || !strings.Contains(err.Error(), "m/003_bad.sql") {
			t.Errorf("%s loadStep(control file) = %v, want command and filename in message", typ, err)
		}
	}
}

// scopeExecutor satisfies the internal executor contract for Apply. The
// history probe reports a missing table so Apply creates it on demand. The
// optional hooks let tests inject failures on specific statements.
type scopeExecutor struct {
	execs    []string
	queryErr error
	execHook func(query string) error
	rowProbe func(query string) error
}

func (e *scopeExecutor) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	e.execs = append(e.execs, query)

	if e.execHook != nil {
		return nil, e.execHook(query)
	}

	return nil, nil
}

func (e *scopeExecutor) QueryContext(_ context.Context, query string, _ ...any) (*scan.Rows, error) {
	if e.queryErr != nil {
		return nil, e.queryErr
	}

	return nil, errors.New("Table 'app.migrations' doesn't exist")
}

func (*scopeExecutor) QueryRowContext(context.Context, string, ...any) scan.Row {
	return scan.ErrorRow(errors.New("unexpected managed object probe"))
}

func (e *scopeExecutor) ran(sub string) bool {
	for _, q := range e.execs {
		if strings.Contains(q, sub) {
			return true
		}
	}

	return false
}

type scopeTx struct {
	execs     []string
	execErr   error
	commitErr error
	rollErr   error
	committed bool
	rolled    bool
}

func (tx *scopeTx) ExecContext(_ context.Context, query string, _ ...any) (result.Result, error) {
	tx.execs = append(tx.execs, query)

	if tx.execErr != nil {
		return nil, tx.execErr
	}

	return nil, nil
}

func (tx *scopeTx) Commit(context.Context) error {
	if tx.commitErr != nil {
		return tx.commitErr
	}

	tx.committed = true

	return nil
}

func (tx *scopeTx) Rollback(context.Context) error {
	tx.rolled = true

	return tx.rollErr
}

func (tx *scopeTx) ran(sub string) bool {
	for _, q := range tx.execs {
		if strings.Contains(q, sub) {
			return true
		}
	}

	return false
}

type scopeBeginner struct {
	txs       []*scopeTx
	beginErr  error
	commitErr error
	rollErr   error
}

func (b *scopeBeginner) Begin(context.Context) (Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}

	tx := &scopeTx{commitErr: b.commitErr, rollErr: b.rollErr}
	b.txs = append(b.txs, tx)

	return tx, nil
}

func mustScopeStep(t *testing.T, fsys fs.FS, version int, direction, name string, transactional bool) Step {
	t.Helper()

	f, err := sqlfile.NewSQLFile(fsys, name)

	if err != nil {
		t.Fatalf("NewSQLFile(%q) = %v", name, err)
	}

	return Step{Version: version, Direction: direction, File: f, Transactional: transactional}
}

func TestApplyMySQLScopeSelection(t *testing.T) {
	fsys := fstest.MapFS{
		"m/002_seed.sql": &fstest.MapFile{Data: []byte("INSERT INTO users VALUES (1);")},
		"m/001_users.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE users (id INT PRIMARY KEY);"),
		},
	}

	target := 1

	t.Run("transactional file runs in file scope", func(t *testing.T) {
		exec := &scopeExecutor{}
		begin := &scopeBeginner{}

		plan := &Plan{
			CurrentVersion: 0,
			TargetVersion:  1,
			Steps:          []Step{mustScopeStep(t, fsys, 1, DirectionUp, "m/002_seed.sql", true)},
		}

		cfg := &config.MigrationsConfig{FS: fsys, TargetVersion: &target}

		err := Apply(context.Background(), exec, begin, plan, cfg, dialect.For(config.DBTypeMySQL))

		if err != nil {
			t.Fatalf("Apply() = %v", err)
		}

		if len(begin.txs) != 1 || !begin.txs[0].committed {
			t.Fatalf("file scope = %d begin(s) committed, want 1", len(begin.txs))
		}

		if !begin.txs[0].ran("INSERT INTO users VALUES (1)") {
			t.Errorf("file text did not run inside the transaction: %q", begin.txs[0].execs)
		}

		if exec.ran("INSERT INTO users VALUES (1)") {
			t.Errorf("file text ran outside the transaction: %q", exec.execs)
		}
	})

	t.Run("non-transactional file runs without transaction", func(t *testing.T) {
		exec := &scopeExecutor{}
		begin := &scopeBeginner{}

		plan := &Plan{
			CurrentVersion: 0,
			TargetVersion:  1,
			Steps:          []Step{mustScopeStep(t, fsys, 1, DirectionUp, "m/001_users.sql", false)},
		}

		cfg := &config.MigrationsConfig{FS: fsys, TargetVersion: &target}

		err := Apply(context.Background(), exec, begin, plan, cfg, dialect.For(config.DBTypeMySQL))

		if err != nil {
			t.Fatalf("Apply() = %v", err)
		}

		if len(begin.txs) != 0 {
			t.Fatalf("non-transactional file started %d transaction(s), want 0", len(begin.txs))
		}

		if !exec.ran("CREATE TABLE users") {
			t.Errorf("file text did not run on the executor: %q", exec.execs)
		}
	})
}
