package selects

import (
	"testing"

	"github.com/mineot/krios/internal/dialects"
)

func TestFromChain(t *testing.T) {
	for _, tc := range []struct {
		dialect dialects.Dialect
		want    string
	}{
		{dialects.Postgres, `"public"."users" AS "u"`},
		{dialects.SQLite, `"public"."users" AS "u"`},
		{dialects.Mysql, "`public`.`users` AS `u`"},
	} {
		t.Run(string(tc.dialect), func(t *testing.T) {
			s := NewSelect(tc.dialect)
			if got := s.From("users").As("u").Schema("public"); got != s {
				t.Fatal("chain returned a different builder")
			}
			if got := s.Build(); got != tc.want || s.Err() != nil {
				t.Fatalf("Build() = %q, Err() = %v", got, s.Err())
			}
			s.paginate = []string{"old"}
			s.From("orders")
			if got := s.Build(); got == "" || s.Err() != nil || s.from.Name != "orders" || s.paginate != nil {
				t.Fatalf("source replacement failed: %q, %v", got, s.Err())
			}
			q := dialects.Quoter{Dialect: tc.dialect}

			if s.from.Build() != q.Quotes("orders") {
				t.Fatal("previous alias or schema survived source replacement")
			}
		})
	}
}

func TestFirstErrorStopsChain(t *testing.T) {
	for name, fail := range map[string]func(*Select){
		"alias without source":  func(s *Select) { s.As("u") },
		"schema without source": func(s *Select) { s.Schema("public") },
		"build without source":  func(s *Select) { s.Build() },
		"empty source":          func(s *Select) { s.From("") },
		"empty alias":           func(s *Select) { s.From("users").As("") },
		"empty schema":          func(s *Select) { s.From("users").Schema("") },
	} {
		t.Run(name, func(t *testing.T) {
			s := NewSelect(dialects.Postgres)
			fail(s)
			first := s.Err()
			if first == nil {
				t.Fatal("expected an error")
			}
			previous := s.from
			s.From("orders").As("o").Schema("other")
			if s.Build() != "" || s.Err() != first || s.from != previous {
				t.Fatal("builder did not retain its first error and source")
			}
		})
	}
}

func TestInvalidDialect(t *testing.T) {
	for _, dialect := range []dialects.Dialect{"", "invalid"} {
		s := NewSelect(dialect)
		if s.Err() == nil || s.From("users").Build() != "" {
			t.Fatal("expected dialect error")
		}
	}
}
