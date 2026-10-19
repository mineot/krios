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
		sql  string
		want bool
	}{
		{"INSERT INTO t VALUES (1)", true},
		{"  \n\t update t set a = 1", true},
		{"-- comment\nDELETE FROM t", true},
		{"/* leading */ SELECT 1", true},
		{"/* only a comment */", true},
		{"", true},
		{"CREATE TABLE t (id INT)", false},
		{"create table t (id INT)", false},
		{"\n-- c\n/* b */\nALTER TABLE t ADD c INT", false},
		{"DROP TABLE t", false},
		{"TRUNCATE TABLE t", false},
		{"ANALYZE TABLE t", false},
		{"\n-- comment\nanalyze table t", false},
		{"/* unterminated", false},
	}

	for _, tc := range cases {
		got, err := d.IsTransactionalStatement(tc.sql)
		if tc.sql == "/* unterminated" {
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

	// The keyword list is conservative on every dialect, not only MySQL.
	pg := For(config.DBTypePostgres).(MigrationDialect)
	if ok, err := pg.IsTransactionalStatement("CREATE INDEX i ON t (a)"); err != nil || ok {
		t.Errorf("postgres IsTransactionalStatement(CREATE INDEX) = %v, %v", ok, err)
	}
}
