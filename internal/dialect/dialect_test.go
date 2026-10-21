package dialect

import (
	"testing"

	"github.com/mineot/krios/internal/config"
)

func TestForReturnsDialectPerType(t *testing.T) {
	cases := []struct {
		typ   config.DBType
		name  string
		ph    string
		qual  string
		ident string
	}{
		{config.DBTypeSQLite, "sqlite", "?", `"`, `"users"."id"`},
		{config.DBTypeMySQL, "mysql", "?", "`", "`users`.`id`"},
		{config.DBTypePostgres, "postgres", "$1", `"`, `"users"."id"`},
	}

	for _, tc := range cases {
		d := For(tc.typ)
		if d == nil {
			t.Fatalf("For(%q) = nil", tc.typ)
		}
		if got := d.Name(); got != tc.name {
			t.Errorf("Name() = %q, want %q", got, tc.name)
		}
		if got := d.Placeholder(1); got != tc.ph {
			t.Errorf("Placeholder(1) = %q, want %q", got, tc.ph)
		}
		if got := d.QuoteIdent("users.id"); got != tc.ident {
			t.Errorf("QuoteIdent(users.id) = %q, want %q", got, tc.ident)
		}
	}

	if For(config.DBType("oracle")) != nil {
		t.Error("For(oracle) must be nil")
	}
}

func TestPostgresPlaceholderSequence(t *testing.T) {
	d := For(config.DBTypePostgres)
	for i, want := range []string{"$1", "$2", "$10"} {
		n := i + 1
		if i == 2 {
			n = 10
		}
		if got := d.Placeholder(n); got != want {
			t.Errorf("Placeholder(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestQuoteIdentQualifiedName(t *testing.T) {
	cases := []struct {
		typ  config.DBType
		name string
		want string
	}{
		{config.DBTypeSQLite, "public.users", `"public"."users"`},
		{config.DBTypeMySQL, "app.tasks", "`app`.`tasks`"},
		{config.DBTypePostgres, "public.users", `"public"."users"`},
	}
	for _, tc := range cases {
		if got := For(tc.typ).QuoteIdent(tc.name); got != tc.want {
			t.Errorf("%s QuoteIdent(%q) = %q, want %q", tc.typ, tc.name, got, tc.want)
		}
	}
}

func TestQuoteIdentEscapesDelimiters(t *testing.T) {
	cases := []struct {
		typ  config.DBType
		name string
		want string
	}{
		{config.DBTypeSQLite, `weird"name`, `"weird""name"`},
		{config.DBTypeSQLite, `public.we"ird`, `"public"."we""ird"`},
		{config.DBTypePostgres, `weird"name`, `"weird""name"`},
		{config.DBTypeMySQL, "weird`name", "`weird``name`"},
		{config.DBTypeMySQL, "app.we`ird", "`app`.`we``ird`"},
	}
	for _, tc := range cases {
		if got := For(tc.typ).QuoteIdent(tc.name); got != tc.want {
			t.Errorf("%s QuoteIdent(%q) = %q, want %q", tc.typ, tc.name, got, tc.want)
		}
	}
}

func TestTransactionalDDLByDialect(t *testing.T) {
	if !For(config.DBTypeSQLite).(MigrationDialect).SupportsTransactionalDDL() {
		t.Error("SQLite must support transactional DDL")
	}
	if !For(config.DBTypePostgres).(MigrationDialect).SupportsTransactionalDDL() {
		t.Error("PostgreSQL must support transactional DDL")
	}
	if For(config.DBTypeMySQL).(MigrationDialect).SupportsTransactionalDDL() {
		t.Error("MySQL must not claim universal transactional DDL")
	}
}

func TestIsTransactionalStatement(t *testing.T) {
	d := For(config.DBTypeMySQL).(MigrationDialect)

	cases := []struct {
		sql   string
		want  bool
		isErr bool
	}{
		{"INSERT INTO t VALUES (1)", true, false},
		{"  \n\t update t set a = 1", true, false},
		{"-- comment\nDELETE FROM t", true, false},
		{"/* leading */ SELECT 1", true, false},
		{"REPLACE INTO t VALUES (1)", true, false},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true, false},
		{"/* only a comment */", true, false},
		{"", true, false},
		{"CREATE TABLE t (id INT)", false, false},
		{"create table t (id INT)", false, false},
		{"\n-- c\n/* b */\nALTER TABLE t ADD c INT", false, false},
		{"DROP TABLE t", false, false},
		{"TRUNCATE TABLE t", false, false},
		{"ANALYZE TABLE t", false, false},
		{"\n-- comment\nanalyze table t", false, false},
		{"GRANT ALL ON t TO u", false, false},
		{"LOAD DATA INFILE 'f' INTO TABLE t", false, false},
		{"FLUSH TABLES", false, false},
		{"SET NAMES utf8mb4", false, false},
		{"PRAGMA foreign_keys = OFF", false, false},
		{"CALL sp()", false, false},
		{`"CREATE" TABLE t (id INT)`, false, false},
		{"'CREATE TABLE t'", false, false},
		{"`INSERT` INTO t VALUES (1)", false, false},
		{"BEGIN", false, true},
		{"START TRANSACTION", false, true},
		{"\n-- c\ncommit", false, true},
		{"SAVEPOINT sp1", false, true},
		{"ROLLBACK", false, true},
		{"RELEASE SAVEPOINT sp1", false, true},
		{"/* unterminated", false, true},
	}

	for _, tc := range cases {
		got, err := d.IsTransactionalStatement(tc.sql)

		if tc.isErr {
			if err == nil {
				t.Errorf("IsTransactionalStatement(%q) error = nil, want error", tc.sql)
			}
			continue
		}

		if err != nil {
			t.Errorf("IsTransactionalStatement(%q) error = %v", tc.sql, err)
			continue
		}
		if got != tc.want {
			t.Errorf("IsTransactionalStatement(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}

	// The keyword list is conservative on every dialect, not only MySQL,
	// and control statements are rejected everywhere (C03-P02 D10/D11).
	for _, typ := range []config.DBType{config.DBTypeSQLite, config.DBTypePostgres} {
		md := For(typ).(MigrationDialect)
		if ok, err := md.IsTransactionalStatement("CREATE INDEX i ON t (a)"); err != nil || ok {
			t.Errorf("%s IsTransactionalStatement(CREATE INDEX) = %v, %v", typ, ok, err)
		}
		if _, err := md.IsTransactionalStatement("BEGIN"); err == nil {
			t.Errorf("%s IsTransactionalStatement(BEGIN) error = nil, want error", typ)
		}
	}
}
