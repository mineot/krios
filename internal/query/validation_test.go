package query

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/entity"
)

func TestValidationFullChainPerDialect(t *testing.T) {
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

			query, args, err := New(nil, d, acc).
				Only("name").
				Fields(
					acc.Col("status"),
					acc.Col("id").Count().As("total"),
					tsk.Col("id").Sum().As("sum"),
				).
				Distinct().
				Join(tsk, LeftJoin, tsk.Col("account").Eq(acc.Col("id"))).
				Where(
					acc.Col("status").Eq("active"),
					tsk.Col("title").Like(entity.Contains, "50%!_"),
					acc.Col("id").In(int64(1), int64(2)),
				).
				GroupBy(acc.Col("name"), acc.Col("status")).
				OrderBy(acc.Col("id").Count().Desc(), tsk.Col("id").Sum().Asc()).
				Limit(10).
				Offset(20).
				Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			want := fmt.Sprintf(`SELECT DISTINCT %s, %s, COUNT(%s) AS %s, SUM(%s) AS %s FROM %s `+
				`LEFT JOIN %s ON %s = %s WHERE %s = %s AND %s LIKE %s ESCAPE '!' AND %s IN (%s, %s) `+
				`GROUP BY %s, %s ORDER BY COUNT(%s) DESC, SUM(%s) ASC LIMIT %s OFFSET %s`,
				d.QuoteIdent("account")+"."+d.QuoteIdent("name"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"), d.QuoteIdent("total"),
				d.QuoteIdent("task")+"."+d.QuoteIdent("id"), d.QuoteIdent("sum"),
				d.QuoteIdent("account"),
				d.QuoteIdent("task"),
				d.QuoteIdent("task")+"."+d.QuoteIdent("account"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("status"), d.Placeholder(1),
				d.QuoteIdent("task")+"."+d.QuoteIdent("title"), d.Placeholder(2),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
				d.Placeholder(3), d.Placeholder(4),
				d.QuoteIdent("account")+"."+d.QuoteIdent("name"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"),
				d.QuoteIdent("task")+"."+d.QuoteIdent("id"),
				d.Placeholder(5), d.Placeholder(6))
			if query != want {
				t.Errorf("sql = %q, want %q", query, want)
			}

			wantArgs := []any{"active", "%50!%!!!_%", int64(1), int64(2), 10, 20}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Errorf("args = %v, want %v", args, wantArgs)
			}
		})
	}
}

func TestValidationCountQueryPerDialect(t *testing.T) {
	dialects := []config.DBType{
		config.DBTypeSQLite,
		config.DBTypeMySQL,
		config.DBTypePostgres,
	}

	for _, typ := range dialects {
		d := dialect.For(typ)
		t.Run(string(typ), func(t *testing.T) {
			acc := accountEntity(t)

			query, args, err := New(nil, d, acc).
				Fields(acc.Col("status"), acc.Col("id").Count().As("total")).
				Distinct().
				Where(
					acc.Col("name").Like(entity.StartsWith, "a"),
					acc.Col("id").In(int64(1), int64(2)),
				).
				GroupBy(acc.Col("status")).
				OrderBy(acc.Col("id").Count().Desc()).
				Limit(5).
				Offset(10).
				countQuery()
			if err != nil {
				t.Fatalf("countQuery() error = %v", err)
			}

			want := fmt.Sprintf(`SELECT COUNT(*) FROM (SELECT DISTINCT %s, COUNT(%s) AS %s FROM %s `+
				`WHERE %s LIKE %s ESCAPE '!' AND %s IN (%s, %s) GROUP BY %s) AS %s`,
				d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"), d.QuoteIdent("total"),
				d.QuoteIdent("account"),
				d.QuoteIdent("account")+"."+d.QuoteIdent("name"), d.Placeholder(1),
				d.QuoteIdent("account")+"."+d.QuoteIdent("id"), d.Placeholder(2), d.Placeholder(3),
				d.QuoteIdent("account")+"."+d.QuoteIdent("status"),
				d.QuoteIdent("krios_total"))
			if query != want {
				t.Errorf("sql = %q, want %q", query, want)
			}

			wantArgs := []any{"a%", int64(1), int64(2)}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Errorf("args = %v, want %v", args, wantArgs)
			}
		})
	}
}

func TestValidationCountQueryIgnoresBuilderPagination(t *testing.T) {
	acc := accountEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	plain, args, err := New(nil, d, acc).
		Where(acc.Col("status").Eq("active")).
		OrderBy(acc.Col("id").Desc()).
		countQuery()
	if err != nil {
		t.Fatalf("countQuery() error = %v", err)
	}
	limited, limitedArgs, err := New(nil, d, acc).
		Where(acc.Col("status").Eq("active")).
		OrderBy(acc.Col("id").Desc()).
		Limit(1).
		Offset(5).
		countQuery()
	if err != nil {
		t.Fatalf("countQuery() with limit error = %v", err)
	}
	if plain != limited {
		t.Errorf("countQuery() changed with limit/offset: %q != %q", limited, plain)
	}
	if !reflect.DeepEqual(args, limitedArgs) {
		t.Errorf("countQuery() args changed with limit/offset: %v != %v", limitedArgs, args)
	}
}
