package write

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
	ID   int64 `krios:"id,pk"`
	Name string
	Bio  *string
}

type patch struct {
	Bio  *string
	Note *string
}

type checkedStatus string

func (s checkedStatus) Validate() error {
	if strings.TrimSpace(string(s)) == "" {
		return errors.New("status is required")
	}
	return nil
}

type order struct {
	ID     int64 `krios:"id,pk"`
	Status checkedStatus
}

func entityOf[T any](t *testing.T) *entity.Entity {
	t.Helper()
	ent, err := entity.EntityOf[T]()
	if err != nil {
		t.Fatalf("EntityOf: %v", err)
	}
	return ent
}

func dialects() []struct {
	typ config.DBType
	d   dialect.Dialect
} {
	all := []struct {
		typ config.DBType
		d   dialect.Dialect
	}{
		{config.DBTypeSQLite, dialect.For(config.DBTypeSQLite)},
		{config.DBTypeMySQL, dialect.For(config.DBTypeMySQL)},
		{config.DBTypePostgres, dialect.For(config.DBTypePostgres)},
	}
	return all
}

func ptr(s string) *string { return &s }

func render(t *testing.T, build func() (string, []any, error)) (string, []any) {
	t.Helper()
	query, args, err := build()
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	return query, args
}

func assertQuery(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("Build() query = %q, want %q", got, want)
	}
}

func assertArgs(t *testing.T, got, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Build() args = %v, want %v", got, want)
	}
	for i := range got {
		if fmt.Sprint(got[i]) != fmt.Sprint(want[i]) {
			t.Errorf("Build() args[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestInsertBuildPerDialect(t *testing.T) {
	acc := entityOf[account](t)

	for _, tc := range dialects() {
		t.Run(string(tc.typ), func(t *testing.T) {
			d := tc.d
			table := d.QuoteIdent("account")

			cases := []struct {
				name  string
				build func(b *Insert) *Insert
				want  string
				args  []any
			}{
				{
					name:  "single row with nil pointer binding NULL",
					build: func(b *Insert) *Insert { return b.SetValues(&account{ID: 1, Name: "ana", Bio: nil}) },
					want:  "INSERT INTO " + table + " (" + d.QuoteIdent("id") + ", " + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio") + ") VALUES (" + d.Placeholder(1) + ", " + d.Placeholder(2) + ", " + d.Placeholder(3) + ")",
					args:  []any{int64(1), "ana", nil},
				},
				{
					name: "omitted columns use database default",
					build: func(b *Insert) *Insert {
						return b.Omit("id").SetValues(&account{Name: "bia"})
					},
					want: "INSERT INTO " + table + " (" + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio") + ") VALUES (" + d.Placeholder(1) + ", " + d.Placeholder(2) + ")",
					args: []any{"bia", nil},
				},
				{
					name: "returning lists every mapped column",
					build: func(b *Insert) *Insert {
						return b.Omit("id").SetValues(&account{Name: "bia"}).Returning()
					},
					want: "INSERT INTO " + table + " (" + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio") + ") VALUES (" + d.Placeholder(1) + ", " + d.Placeholder(2) + ") RETURNING " +
						d.QuoteIdent("id") + ", " + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio"),
					args: []any{"bia", nil},
				},
			}

			if string(tc.typ) == "mysql" {
				cases = cases[:len(cases)-1]
			}

			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					query, args := render(t, func() (string, []any, error) {
						return c.build(NewInsert(nil, d, acc)).Build()
					})
					assertQuery(t, query, c.want)
					assertArgs(t, args, c.args)
				})
			}
		})
	}
}

func TestUpdateBuildPerDialect(t *testing.T) {
	acc := entityOf[account](t)

	for _, tc := range dialects() {
		t.Run(string(tc.typ), func(t *testing.T) {
			d := tc.d
			table := d.QuoteIdent("account")

			cases := []struct {
				name  string
				build func(b *Update) *Update
				want  string
				args  []any
			}{
				{
					name: "set skips nil pointers and where follows in textual order",
					build: func(b *Update) *Update {
						return b.
							SetValues(&account{ID: 1, Name: "ana", Bio: nil}).
							Where(acc.Col("id").Eq(1))
					},
					want: "UPDATE " + table + " SET " + d.QuoteIdent("id") + " = " + d.Placeholder(1) + ", " + d.QuoteIdent("name") + " = " + d.Placeholder(2) +
						" WHERE " + table + "." + d.QuoteIdent("id") + " = " + d.Placeholder(3),
					args: []any{int64(1), "ana", int64(1)},
				},
				{
					name: "omit removes columns from assignment",
					build: func(b *Update) *Update {
						return b.
							SetValues(&account{ID: 1, Name: "ana", Bio: ptr("b")}).
							Omit("id").
							Where(acc.Col("id").Eq(1))
					},
					want: "UPDATE " + table + " SET " + d.QuoteIdent("name") + " = " + d.Placeholder(1) + ", " + d.QuoteIdent("bio") + " = " + d.Placeholder(2) +
						" WHERE " + table + "." + d.QuoteIdent("id") + " = " + d.Placeholder(3),
					args: []any{"ana", "b", int64(1)},
				},
				{
					name: "conditions combine with AND and returning appends last",
					build: func(b *Update) *Update {
						return b.
							SetValues(&account{Name: "ana"}).
							Omit("id").
							Where(acc.Col("name").Eq("a"), acc.Col("bio").IsNotNull()).
							Returning()
					},
					want: "UPDATE " + table + " SET " + d.QuoteIdent("name") + " = " + d.Placeholder(1) +
						" WHERE " + table + "." + d.QuoteIdent("name") + " = " + d.Placeholder(2) + " AND " + table + "." + d.QuoteIdent("bio") + " IS NOT NULL" +
						" RETURNING " + d.QuoteIdent("id") + ", " + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio"),
					args: []any{"ana", "a"},
				},
			}

			for _, c := range cases {
				if string(tc.typ) == "mysql" && strings.Contains(c.want, "RETURNING") {
					continue
				}

				t.Run(c.name, func(t *testing.T) {
					query, args := render(t, func() (string, []any, error) {
						return c.build(NewUpdate(nil, d, acc)).Build()
					})
					assertQuery(t, query, c.want)
					assertArgs(t, args, c.args)
				})
			}
		})
	}
}

func TestDeleteBuildPerDialect(t *testing.T) {
	acc := entityOf[account](t)

	for _, tc := range dialects() {
		t.Run(string(tc.typ), func(t *testing.T) {
			d := tc.d
			table := d.QuoteIdent("account")

			query, args := render(t, NewDelete(nil, d, acc).Where(acc.Col("id").Eq(1)).Build)
			want := "DELETE FROM " + table + " WHERE " + table + "." + d.QuoteIdent("id") + " = " + d.Placeholder(1)
			assertQuery(t, query, want)
			assertArgs(t, args, []any{int64(1)})

			if string(tc.typ) == "mysql" {
				return
			}

			query, args = render(t, NewDelete(nil, d, acc).
				Where(acc.Col("name").Eq("x")).
				Returning().
				Build)
			want = "DELETE FROM " + table + " WHERE " + table + "." + d.QuoteIdent("name") + " = " + d.Placeholder(1) +
				" RETURNING " + d.QuoteIdent("id") + ", " + d.QuoteIdent("name") + ", " + d.QuoteIdent("bio")
			assertQuery(t, query, want)
			assertArgs(t, args, []any{"x"})
		})
	}
}

func TestReturningUnsupportedOnMySQL(t *testing.T) {
	acc := entityOf[account](t)
	d := dialect.For(config.DBTypeMySQL)

	builders := map[string]func() (string, []any, error){
		"insert": NewInsert(nil, d, acc).SetValues(&account{Name: "x"}).Returning().Build,
		"update": NewUpdate(nil, d, acc).SetValues(&account{Name: "x"}).Where(acc.Col("id").Eq(1)).Returning().Build,
		"delete": NewDelete(nil, d, acc).Where(acc.Col("id").Eq(1)).Returning().Build,
	}

	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			_, _, err := build()
			if !errors.Is(err, errs.ErrInvalidQuery) {
				t.Fatalf("Build() = %v, want ErrInvalidQuery", err)
			}
			if !strings.Contains(err.Error(), "mysql") {
				t.Errorf("Build() = %v, want dialect name in message", err)
			}
		})
	}
}

type task struct {
	ID    int64 `krios:"id,pk"`
	Title string
}

func TestConstructionErrors(t *testing.T) {
	acc := entityOf[account](t)
	tsk := entityOf[task](t)
	pat := entityOf[patch](t)
	ord := entityOf[order](t)
	d := dialect.For(config.DBTypeSQLite)

	cases := []struct {
		name  string
		build func() (string, []any, error)
		want  error
	}{
		{
			name:  "insert without SetValues",
			build: NewInsert(nil, d, acc).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "insert with every column omitted",
			build: NewInsert(nil, d, acc).Omit("id", "name", "bio").SetValues(&account{Name: "x"}).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "insert with unknown omit column",
			build: NewInsert(nil, d, acc).Omit("nope").SetValues(&account{Name: "x"}).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "insert with duplicated omit column",
			build: NewInsert(nil, d, acc).Omit("name", "name").SetValues(&account{Name: "x"}).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "insert with record of another type",
			build: NewInsert(nil, d, acc).SetValues(&patch{}).Build,
			want:  errs.ErrInvalidEntity,
		},
		{
			name:  "insert with nil record",
			build: NewInsert(nil, d, acc).SetValues(nil).Build,
			want:  errs.ErrInvalidEntity,
		},
		{
			name:  "insert with failing field validation",
			build: NewInsert(nil, d, ord).SetValues(&order{Status: " "}).Build,
			want:  errs.ErrValidation,
		},
		{
			name:  "insert without entity",
			build: NewInsert(nil, d, nil).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "update without SetValues",
			build: NewUpdate(nil, d, acc).Where(acc.Col("id").Eq(1)).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "update without Where",
			build: NewUpdate(nil, d, acc).SetValues(&account{Name: "x"}).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "update with empty Where call",
			build: NewUpdate(nil, d, acc).SetValues(&account{Name: "x"}).Where().Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "update with no column to write",
			build: NewUpdate(nil, d, pat).SetValues(&patch{}).Where(pat.Col("bio").IsNull()).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "update with condition on foreign table",
			build: NewUpdate(nil, d, acc).SetValues(&account{Name: "x"}).Where(tsk.Col("id").Eq(1)).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "delete without Where",
			build: NewDelete(nil, d, acc).Build,
			want:  errs.ErrInvalidQuery,
		},
		{
			name:  "delete with condition on foreign table",
			build: NewDelete(nil, d, acc).Where(tsk.Col("id").Eq(1)).Build,
			want:  errs.ErrInvalidQuery,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := c.build()
			if !errors.Is(err, c.want) {
				t.Errorf("Build() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestSetValuesFirstErrorWins(t *testing.T) {
	acc := entityOf[account](t)
	d := dialect.For(config.DBTypeSQLite)

	b := NewInsert(nil, d, acc)
	b.SetValues(&patch{}) // wrong type: remembered
	b.SetValues(&account{Name: "x"})

	if _, _, err := b.Build(); !errors.Is(err, errs.ErrInvalidEntity) {
		t.Errorf("Build() = %v, want remembered ErrInvalidEntity", err)
	}

	b = NewInsert(nil, d, acc)
	b.SetValues(&account{Name: "x"})
	b.SetValues(nil)

	if _, _, err := b.Build(); !errors.Is(err, errs.ErrInvalidEntity) {
		t.Errorf("Build() = %v, want ErrInvalidEntity from later failure", err)
	}
}

func TestExecutionModeRejections(t *testing.T) {
	acc := entityOf[account](t)
	d := dialect.For(config.DBTypeSQLite)

	t.Run("Exec after Returning", func(t *testing.T) {
		_, err := NewInsert(nil, d, acc).SetValues(&account{Name: "x"}).Returning().Exec(t.Context())
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("Exec() = %v, want ErrInvalidQuery", err)
		}
	})

	t.Run("One without Returning", func(t *testing.T) {
		err := NewInsert(nil, d, acc).SetValues(&account{Name: "x"}).One(t.Context(), &account{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("One() = %v, want ErrInvalidQuery", err)
		}
	})

	t.Run("All without Returning", func(t *testing.T) {
		err := NewUpdate(nil, d, acc).SetValues(&account{Name: "x"}).Where(acc.Col("id").Eq(1)).All(t.Context(), &[]account{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("All() = %v, want ErrInvalidQuery", err)
		}
		err = NewDelete(nil, d, acc).Where(acc.Col("id").Eq(1)).All(t.Context(), &[]account{})
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("delete All() = %v, want ErrInvalidQuery", err)
		}
	})

	t.Run("missing executor", func(t *testing.T) {
		_, err := NewInsert(nil, d, acc).SetValues(&account{Name: "x"}).Exec(t.Context())
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Errorf("Exec() = %v, want ErrInvalidQuery", err)
		}
	})
}
