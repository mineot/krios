package entity

import (
	"errors"
	"strings"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

func renderExpr(t *testing.T, d dialect.Dialect, e Expr) string {
	t.Helper()

	var (
		b    strings.Builder
		args []any
	)
	if err := e.ExprInto(d, &b, &args); err != nil {
		t.Fatalf("ExprInto() error = %v", err)
	}
	return b.String()
}

func TestExprRendering(t *testing.T) {
	ent := mustUserEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"count all", Count(), "COUNT(*)"},
		{"count column", ent.Col("id").Count(), `COUNT("metadata_user"."id")`},
		{"sum", ent.Col("id").Sum(), `SUM("metadata_user"."id")`},
		{"max", ent.Col("id").Max(), `MAX("metadata_user"."id")`},
		{"min", ent.Col("id").Min(), `MIN("metadata_user"."id")`},
		{"column", ent.Col("name"), `"metadata_user"."name"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderExpr(t, d, tc.expr); got != tc.want {
				t.Errorf("ExprInto() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExprAliasAndEntities(t *testing.T) {
	ent := mustUserEntity(t)
	agg := ent.Col("id").Count().As(" total ")

	if agg.Alias() != " total " {
		t.Errorf("Alias() = %q, want the configured alias", agg.Alias())
	}
	if ent.Col("id").Alias() != "" {
		t.Error("Column.Alias() must be empty")
	}
	if ent.Col("id").Name() != "id" {
		t.Errorf("Column.Name() = %q, want id", ent.Col("id").Name())
	}

	entities := agg.Entities()
	if len(entities) != 1 || entities[0] != ent {
		t.Errorf("Aggregate.Entities() = %v, want the owning entity", entities)
	}
	if Count().Entities() != nil {
		t.Error("Count().Entities() must be nil")
	}
	if entities := ent.Col("id").Entities(); len(entities) != 1 || entities[0] != ent {
		t.Errorf("Column.Entities() = %v, want the owning entity", entities)
	}
	if entities := (Column{}).Entities(); entities != nil {
		t.Errorf("nil Column.Entities() = %v, want nil", entities)
	}
}

func TestSortRendering(t *testing.T) {
	ent := mustUserEntity(t)
	d := dialect.For(config.DBTypePostgres)

	cases := []struct {
		name string
		sort Sort
		want string
	}{
		{"column asc", ent.Col("name").Asc(), `"metadata_user"."name" ASC`},
		{"column desc", ent.Col("name").Desc(), `"metadata_user"."name" DESC`},
		{"aggregate desc", ent.Col("id").Count().Desc(), `COUNT("metadata_user"."id") DESC`},
		{"aggregate asc", ent.Col("id").Sum().Asc(), `SUM("metadata_user"."id") ASC`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				b    strings.Builder
				args []any
			)
			if err := tc.sort.ExprInto(d, &b, &args); err != nil {
				t.Fatalf("ExprInto() error = %v", err)
			}
			if got := b.String(); got != tc.want {
				t.Errorf("ExprInto() = %q, want %q", got, tc.want)
			}
		})
	}

	if entities := ent.Col("id").Desc().Entities(); len(entities) != 1 || entities[0] != ent {
		t.Errorf("Sort.Entities() = %v, want the owning entity", entities)
	}
}

func TestExprErrors(t *testing.T) {
	ent := mustUserEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	var (
		b    strings.Builder
		args []any
	)
	if err := ent.Col("missing").Count().ExprInto(d, &b, &args); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("unknown column error = %v, want ErrInvalidQuery", err)
	}
	if err := ent.Col("id").Count().ExprInto(nil, &b, &args); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("nil dialect error = %v, want ErrInvalidQuery", err)
	}
	if err := (Aggregate{}).ExprInto(d, &b, &args); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("zero Aggregate error = %v, want ErrInvalidQuery", err)
	}
	if err := (Sort{}).ExprInto(d, &b, &args); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("zero Sort error = %v, want ErrInvalidQuery", err)
	}
}
