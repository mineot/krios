package query

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
)

type account struct {
	ID     int64 `krios:"id,pk"`
	Name   string
	Status string
}

type task struct {
	ID      int64 `krios:"id,pk"`
	Account int64
	Title   string
}

func accountEntity(t *testing.T) *entity.Entity {
	t.Helper()
	ent, err := entity.EntityOf[account]()
	if err != nil {
		t.Fatalf("EntityOf[account]() = %v", err)
	}
	return ent
}

func taskEntity(t *testing.T) *entity.Entity {
	t.Helper()
	ent, err := entity.EntityOf[task]()
	if err != nil {
		t.Fatalf("EntityOf[task]() = %v", err)
	}
	return ent
}

func allColumnsQuote(d dialect.Dialect, table string, columns ...string) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = d.QuoteIdent(table) + "." + d.QuoteIdent(c)
	}
	return strings.Join(parts, ", ")
}

func TestBuildPerDialect(t *testing.T) {
	dialects := []struct {
		typ config.DBType
	}{
		{config.DBTypeSQLite},
		{config.DBTypeMySQL},
		{config.DBTypePostgres},
	}

	for _, tc := range dialects {
		d := dialect.For(tc.typ)
		t.Run(string(tc.typ), func(t *testing.T) {
			acc := accountEntity(t)
			tsk := taskEntity(t)

			cases := []struct {
				name  string
				build func(s *Select) *Select
				want  string
				args  []any
			}{
				{
					name:  "select all columns",
					build: func(s *Select) *Select { return s },
					want: fmt.Sprintf("SELECT %s FROM %s",
						allColumnsQuote(d, "account", "id", "name", "status"),
						d.QuoteIdent("account")),
					args: nil,
				},
				{
					name: "only subset",
					build: func(s *Select) *Select {
						return s.Only("status", "id")
					},
					want: fmt.Sprintf("SELECT %s, %s FROM %s",
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("account")),
					args: nil,
				},
				{
					name: "where single",
					build: func(s *Select) *Select {
						return s.Where(acc.Col("status").Eq("active"))
					},
					want: fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s",
						allColumnsQuote(d, "account", "id", "name", "status"),
						d.QuoteIdent("account"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.Placeholder(1)),
					args: []any{"active"},
				},
				{
					name: "where multiple and empty where",
					build: func(s *Select) *Select {
						return s.Where(
							acc.Col("status").Eq("active"),
							entity.And(acc.Col("name").Ne("root"), acc.Col("id").Gt(10)),
						)
					},
					want: fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s AND (%s <> %s AND %s > %s)",
						allColumnsQuote(d, "account", "id", "name", "status"),
						d.QuoteIdent("account"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.Placeholder(1),
						d.QuoteIdent("account")+"."+d.QuoteIdent("name"),
						d.Placeholder(2),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.Placeholder(3)),
					args: []any{"active", "root", 10},
				},
				{
					name: "inner join on then where continues placeholders",
					build: func(s *Select) *Select {
						return s.
							Only("id").
							Join(tsk, InnerJoin, tsk.Col("account").Eq(acc.Col("id"))).
							Where(tsk.Col("title").Like(entity.Contains, "krios"))
					},
					want: fmt.Sprintf("SELECT %s FROM %s %s %s ON %s = %s WHERE %s LIKE %s ESCAPE '!'",
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("account"),
						"INNER JOIN", d.QuoteIdent("task"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("account"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("title"),
						d.Placeholder(1)),
					args: []any{"%krios%"},
				},
			}

			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					s := New(nil, d, acc)
					got, args, err := c.build(s).Build()
					if err != nil {
						t.Fatalf("Build() error = %v", err)
					}
					if got != c.want {
						t.Errorf("sql = %q, want %q", got, c.want)
					}
					if len(args) != len(c.args) {
						t.Fatalf("args = %v, want %v", args, c.args)
					}
					for i := range args {
						if args[i] != c.args[i] {
							t.Errorf("args[%d] = %v, want %v", i, args[i], c.args[i])
						}
					}
				})
			}
		})
	}
}

func TestBuildJoinsDeclarationOrder(t *testing.T) {
	acc := accountEntity(t)
	tsk := taskEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	sql, args, err := New(nil, d, acc).
		Join(tsk, LeftJoin, tsk.Col("account").Eq(acc.Col("id"))).
		Where(acc.Col("status").Eq("active")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	want := `SELECT "account"."id", "account"."name", "account"."status" FROM "account" ` +
		`LEFT JOIN "task" ON "task"."account" = "account"."id" WHERE "account"."status" = ?`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != "active" {
		t.Errorf("args = %v, want [active]", args)
	}
}

func TestBuildFailures(t *testing.T) {
	acc := accountEntity(t)
	tsk := taskEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	cases := []struct {
		name  string
		build func(s *Select) *Select
	}{
		{"nil select", nil}, // handled separately below
		{"nil base entity", func(s *Select) *Select { s.ent = nil; return s }},
		{"nil dialect", func(s *Select) *Select { s.dialect = nil; return s }},
		{"only unknown column", func(s *Select) *Select { return s.Only("missing") }},
		{"only duplicate column", func(s *Select) *Select { return s.Only("id", "id") }},
		{"where nil condition", func(s *Select) *Select { return s.Where(entity.Condition{}) }},
		{"where unknown column", func(s *Select) *Select { return s.Where(acc.Col("missing").Eq(1)) }},
		{"where nil operand", func(s *Select) *Select { return s.Where(acc.Col("name").Eq(nil)) }},
		{"where foreign entity", func(s *Select) *Select { return s.Where(tsk.Col("title").Eq("x")) }},
		{"join nil entity", func(s *Select) *Select { return s.Join(nil, InnerJoin, acc.Col("id").Eq(acc.Col("id"))) }},
		{"join duplicate entity", func(s *Select) *Select {
			return s.
				Join(tsk, InnerJoin, tsk.Col("account").Eq(acc.Col("id"))).
				Join(tsk, LeftJoin, tsk.Col("account").Eq(acc.Col("id")))
		}},
		{"join base entity again", func(s *Select) *Select {
			return s.Join(acc, InnerJoin, acc.Col("id").Eq(acc.Col("id")))
		}},
		{"join unknown kind", func(s *Select) *Select {
			return s.Join(tsk, JoinKind(99), tsk.Col("account").Eq(acc.Col("id")))
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "nil select" {
				var s *Select
				if _, _, err := s.Build(); !errors.Is(err, errs.ErrInvalidQuery) {
					t.Errorf("Build() error = %v, want ErrInvalidQuery", err)
				}
				return
			}
			s := c.build(New(nil, d, acc))
			if _, _, err := s.Build(); !errors.Is(err, errs.ErrInvalidQuery) {
				t.Errorf("Build() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestBuildDuplicateEntityDescriptor(t *testing.T) {
	// Two descriptors of the same table are distinct pointers, so the map
	// check must reject the second occurrence by table name.
	d := dialect.For(config.DBTypeSQLite)
	acc := accountEntity(t)
	acc2 := accountEntity(t)

	s := New(nil, d, acc).Join(acc2, InnerJoin, acc2.Col("id").Eq(acc.Col("id")))
	if _, _, err := s.Build(); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Build() error = %v, want ErrInvalidQuery", err)
	}
}

func TestBuildNoExecutorFailsOnExecution(t *testing.T) {
	acc := accountEntity(t)
	s := New(nil, dialect.For(config.DBTypeSQLite), acc).Where(acc.Col("status").Eq("x"))

	if _, err := s.Rows(nil); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Rows() error = %v, want ErrInvalidQuery", err)
	}
	if err := s.One(nil, &account{}); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("One() error = %v, want ErrInvalidQuery", err)
	}
}

func TestBuildDeferredConstructionErrorOnRow(t *testing.T) {
	acc := accountEntity(t)
	// Construction error reaches the consumer through Row/Scan.
	s := New(nil, dialect.For(config.DBTypeSQLite), acc).Where(acc.Col("missing").Eq(1))
	err := s.Row(nil).Scan(&account{})
	if !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Scan() error = %v, want ErrInvalidQuery", err)
	}
}
