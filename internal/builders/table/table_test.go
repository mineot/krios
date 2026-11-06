package table

import (
	"github.com/mineot/krios/internal/dialects"
	"testing"
)

func TestBuildDialects(t *testing.T) {
	for _, tc := range []struct {
		dialect dialects.Dialect
		want    string
	}{
		{dialects.Postgres, `"app"."us""ers` + "`" + `" AS "u""` + "`" + `"`},
		{dialects.SQLite, `"app"."us""ers` + "`" + `" AS "u""` + "`" + `"`},
		{dialects.Mysql, "`app`.`us\"ers``` AS `u\"```"},
	} {
		t.Run(string(tc.dialect), func(t *testing.T) {
			table := &Table{Name: "old.us\"ers`", Dialect: dialects.Postgres}
			table.As("u\"`").Schema("app")
			table.Dialect = tc.dialect
			got := table.Build()
			err := table.Err()
			if err != nil || got != tc.want {
				t.Fatalf("Build() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestBuildQualifiedName(t *testing.T) {
	table := &Table{Name: "public.users", Dialect: dialects.Postgres}
	got := table.Build()
	err := table.Err()
	if err != nil || got != `"public"."users"` {
		t.Fatalf("Build() = %q, %v", got, err)
	}
}

func TestInvalidInputPreservesState(t *testing.T) {
	for _, method := range []string{"As", "Schema"} {
		t.Run(method, func(t *testing.T) {
			table := &Table{Name: "users", Dialect: dialects.Postgres}
			table.As("u").Schema("app")
			if method == "As" {
				table.As("")
			} else {
				table.Schema("")
			}
			first := table.Err()
			if first == nil {
				t.Fatal("expected error")
			}
			table.Dialect = "invalid"
			table.As("changed").Schema("changed")
			if got := table.Build(); got != "" {
				t.Fatalf("Build() = %q", got)
			}
			if table.Err() != first {
				t.Fatal("first error was replaced")
			}
			if table.alias != "u" || table.schema != "app" {
				t.Fatal("state changed after error")
			}
		})
	}
}

func TestBuildInvalidName(t *testing.T) {
	for _, name := range []string{"", ".users", "public.", "public..users"} {
		table := &Table{Name: name, Dialect: dialects.Postgres}
		if got := table.Build(); got != "" || table.Err() == nil {
			t.Fatalf("invalid name %q: %q, %v", name, got, table.Err())
		}
	}
}

func TestDialectErrorIsLocalAndPersistent(t *testing.T) {
	table := &Table{Name: "users", Dialect: "invalid"}
	table.As("u")
	first := table.Err()
	if first == nil {
		t.Fatal("expected dialect error")
	}
	table.Dialect = dialects.Postgres
	if table.Build() != "" || table.Err() != first {
		t.Fatal("error was cleared")
	}
	other := &Table{Name: "users", Dialect: dialects.Postgres}
	if other.Build() != `"users"` || other.Err() != nil {
		t.Fatal("error leaked to another table")
	}
}

func TestQuoterRetainsFirstError(t *testing.T) {
	q := dialects.Quoter{Dialect: "invalid"}
	if got := q.Quotes("users"); got != "" || q.Err() == nil {
		t.Fatalf("Quotes() = %q, Err() = %v", got, q.Err())
	}
	first := q.Err()
	q.Dialect = dialects.Postgres
	if q.Quotes("users") != "" || q.Err() != first {
		t.Fatal("quoter did not retain its first error")
	}
	other := dialects.Quoter{Dialect: dialects.Postgres}
	if other.Quotes("users") != `"users"` || other.Err() != nil {
		t.Fatal("quoter error leaked to another instance")
	}
}
