package query

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
	"github.com/mineot/krios/internal/errs"
)

func TestBuildAggregationsPerDialect(t *testing.T) {
	dialects := []config.DBType{
		config.DBTypeSQLite,
		config.DBTypeMySQL,
		config.DBTypePostgres,
	}

	for _, typ := range dialects {
		d := dialect.For(typ)
		t.Run(string(typ), func(t *testing.T) {
			acc := accountEntity(t)
			tsk := taskEntity(t)

			cases := []struct {
				name  string
				build func(s *Select) *Select
				want  string
				args  []any
			}{
				{
					name: "fields only with aggregate alias",
					build: func(s *Select) *Select {
						return s.Fields(acc.Col("id").Count().As("total"))
					},
					want: fmt.Sprintf(`SELECT COUNT(%s) AS %s FROM %s`,
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("total"),
						d.QuoteIdent("account")),
				},
				{
					name: "only then fields",
					build: func(s *Select) *Select {
						return s.
							Only("name").
							Fields(acc.Col("status"), acc.Col("id").Count().As("total"))
					},
					want: fmt.Sprintf(`SELECT %s, %s, COUNT(%s) AS %s FROM %s`,
						d.QuoteIdent("account")+"."+d.QuoteIdent("name"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("total"),
						d.QuoteIdent("account")),
				},
				{
					name: "count star",
					build: func(s *Select) *Select {
						return s.Fields(entity.Count().As("total"))
					},
					want: fmt.Sprintf(`SELECT COUNT(*) AS %s FROM %s`,
						d.QuoteIdent("total"), d.QuoteIdent("account")),
				},
				{
					name: "distinct group order limit offset",
					build: func(s *Select) *Select {
						return s.
							Fields(
								acc.Col("status"),
								acc.Col("id").Count().As("total"),
							).
							Distinct().
							Where(acc.Col("name").Ne("root")).
							GroupBy(acc.Col("status")).
							OrderBy(acc.Col("id").Count().Desc(), acc.Col("status").Asc()).
							Limit(10).
							Offset(20)
					},
					want: fmt.Sprintf(`SELECT DISTINCT %s, COUNT(%s) AS %s FROM %s WHERE %s <> %s `+
						`GROUP BY %s ORDER BY COUNT(%s) DESC, %s ASC LIMIT %s OFFSET %s`,
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("total"),
						d.QuoteIdent("account"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("name"),
						d.Placeholder(1),
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
						d.Placeholder(2), d.Placeholder(3)),
					args: []any{"root", 10, 20},
				},
				{
					name: "joined aggregate",
					build: func(s *Select) *Select {
						return s.
							Join(tsk, InnerJoin, tsk.Col("account").Eq(acc.Col("id"))).
							Fields(
								acc.Col("id"),
								tsk.Col("title").Count().As("titles"),
								tsk.Col("id").Sum().As("sum"),
								tsk.Col("id").Max().As("max"),
								tsk.Col("id").Min().As("min"),
							).
							GroupBy(acc.Col("id"))
					},
					want: fmt.Sprintf(`SELECT %s, COUNT(%s) AS %s, SUM(%s) AS %s, MAX(%s) AS %s, MIN(%s) AS %s `+
						`FROM %s INNER JOIN %s ON %s = %s GROUP BY %s`,
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("title"), d.QuoteIdent("titles"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("id"), d.QuoteIdent("sum"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("id"), d.QuoteIdent("max"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("id"), d.QuoteIdent("min"),
						d.QuoteIdent("account"), d.QuoteIdent("task"),
						d.QuoteIdent("task")+"."+d.QuoteIdent("account"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
						d.QuoteIdent("account")+"."+d.QuoteIdent("id")),
				},
				{
					name: "limit zero",
					build: func(s *Select) *Select {
						return s.Fields(entity.Count().As("total")).Limit(0)
					},
					want: fmt.Sprintf(`SELECT COUNT(*) AS %s FROM %s LIMIT %s`,
						d.QuoteIdent("total"), d.QuoteIdent("account"), d.Placeholder(1)),
					args: []any{0},
				},
				{
					name: "last limit wins and zero offset is omitted",
					build: func(s *Select) *Select {
						return s.Limit(1).Offset(5).Limit(2).Offset(0)
					},
					want: fmt.Sprintf("SELECT %s FROM %s LIMIT %s",
						allColumnsQuote(d, "account", "id", "name", "status"),
						d.QuoteIdent("account"), d.Placeholder(1)),
					args: []any{2},
				},
			}

			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					got, args, err := c.build(New(nil, d, acc)).Build()
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

func TestBuildAggregationFailures(t *testing.T) {
	acc := accountEntity(t)
	tsk := taskEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	var nilExpr entity.Expr

	cases := []struct {
		name  string
		build func(s *Select) *Select
	}{
		{"nil field", func(s *Select) *Select { return s.Fields(nilExpr) }},
		{"blank alias", func(s *Select) *Select {
			return s.Fields(acc.Col("id").Count().As("   "))
		}},
		{"alias collides with selected column", func(s *Select) *Select {
			return s.Only("id").Fields(acc.Col("name").Count().As("id"))
		}},
		{"duplicate alias", func(s *Select) *Select {
			return s.Fields(
				acc.Col("id").Count().As("total"),
				acc.Col("name").Count().As("total"),
			)
		}},
		{"field foreign entity", func(s *Select) *Select {
			return s.Fields(tsk.Col("title"))
		}},
		{"group by foreign entity", func(s *Select) *Select {
			return s.GroupBy(tsk.Col("title"))
		}},
		{"order by foreign entity", func(s *Select) *Select {
			return s.OrderBy(tsk.Col("title").Asc())
		}},
		{"empty sort", func(s *Select) *Select { return s.OrderBy(entity.Sort{}) }},
		{"negative limit", func(s *Select) *Select { return s.Limit(-1) }},
		{"negative offset", func(s *Select) *Select { return s.Limit(1).Offset(-1) }},
		{"offset without limit", func(s *Select) *Select { return s.Offset(1) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := c.build(New(nil, d, acc)).Build(); !errors.Is(err, errs.ErrInvalidQuery) {
				t.Errorf("Build() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestCountAndPaginateFailuresWithoutExecutor(t *testing.T) {
	acc := accountEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	s := New(nil, d, acc).Fields(entity.Count().As("total"))
	if _, err := s.Count(nil); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Count() error = %v, want ErrInvalidQuery", err)
	}
	if _, err := s.Paginate(nil, 10, 0, &[]account{}); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Paginate() error = %v, want ErrInvalidQuery", err)
	}
	if _, err := s.Paginate(nil, 0, 0, &[]account{}); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Paginate(limit=0) error = %v, want ErrInvalidQuery", err)
	}
	if _, err := s.Paginate(nil, 10, -1, &[]account{}); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Paginate(offset=-1) error = %v, want ErrInvalidQuery", err)
	}
}
