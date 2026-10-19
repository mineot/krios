package entity

import (
	"errors"
	"testing"

	"github.com/mineot/krios/internal/config"
	"github.com/mineot/krios/internal/dialect"
	"github.com/mineot/krios/internal/errs"
)

func mustUserEntity(t *testing.T) *Entity {
	t.Helper()
	ent, err := EntityOf[metadataUser]()
	if err != nil {
		t.Fatalf("EntityOf[metadataUser]() = %v", err)
	}
	return ent
}

func renderCond(t *testing.T, d dialect.Dialect, c Condition) (string, []any) {
	t.Helper()
	sql, args, err := c.Render(d)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return sql, args
}

func TestRenderComparisons(t *testing.T) {
	ent := mustUserEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	cases := []struct {
		name string
		cond Condition
		want string
		args []any
	}{
		{"Eq value", ent.Col("name").Eq("alice"), `"metadata_user"."name" = ?`, []any{"alice"}},
		{"Eq column", ent.Col("id").Eq(ent.Col("bio")), `"metadata_user"."id" = "metadata_user"."bio"`, nil},
		{"Ne", ent.Col("name").Ne("bob"), `"metadata_user"."name" <> ?`, []any{"bob"}},
		{"Lt", ent.Col("id").Lt(10), `"metadata_user"."id" < ?`, []any{10}},
		{"Lte", ent.Col("id").Lte(10), `"metadata_user"."id" <= ?`, []any{10}},
		{"Gt", ent.Col("id").Gt(1), `"metadata_user"."id" > ?`, []any{1}},
		{"Gte", ent.Col("id").Gte(1), `"metadata_user"."id" >= ?`, []any{1}},
		{"IsNull", ent.Col("bio").IsNull(), `"metadata_user"."bio" IS NULL`, nil},
		{"IsNotNull", ent.Col("bio").IsNotNull(), `"metadata_user"."bio" IS NOT NULL`, nil},
		{"In values", ent.Col("id").In(1, 2, 3), `"metadata_user"."id" IN (?, ?, ?)`, []any{1, 2, 3}},
		{"In empty", ent.Col("id").In(), `FALSE`, nil},
		{"Like contains", ent.Col("name").Like(Contains, "an"), `"metadata_user"."name" LIKE ? ESCAPE '!'`, []any{"%an%"}},
		{"Like starts", ent.Col("name").Like(StartsWith, "an"), `"metadata_user"."name" LIKE ? ESCAPE '!'`, []any{"an%"}},
		{"Like ends", ent.Col("name").Like(EndsWith, "an"), `"metadata_user"."name" LIKE ? ESCAPE '!'`, []any{"%an"}},
		{"Like escapes wildcards", ent.Col("name").Like(Contains, "50%!_x"), `"metadata_user"."name" LIKE ? ESCAPE '!'`, []any{"%50!%!!!_x%"}},
		{
			"And group",
			And(ent.Col("name").Eq("a"), ent.Col("bio").IsNull()),
			`("metadata_user"."name" = ? AND "metadata_user"."bio" IS NULL)`,
			[]any{"a"},
		},
		{
			"nested Or in And",
			And(ent.Col("id").Eq(int64(1)), Or(ent.Col("name").Eq("a"), ent.Col("name").Eq("b"))),
			`("metadata_user"."id" = ? AND ("metadata_user"."name" = ? OR "metadata_user"."name" = ?))`,
			[]any{int64(1), "a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := renderCond(t, d, tc.cond)
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
			if len(args) != len(tc.args) {
				t.Fatalf("args = %v, want %v", args, tc.args)
			}
			for i := range args {
				if args[i] != tc.args[i] {
					t.Errorf("args[%d] = %v, want %v", i, args[i], tc.args[i])
				}
			}
		})
	}
}

func TestRenderInEmptyIsAlwaysFalse(t *testing.T) {
	// The empty IN renders the constant FALSE, not a column reference.
	ent := mustUserEntity(t)
	sql, args, err := ent.Col("id").In().Render(dialect.For(config.DBTypeSQLite))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if sql != "FALSE" {
		t.Errorf("sql = %q, want FALSE", sql)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want none", args)
	}
}

func TestRenderPostgresPlaceholders(t *testing.T) {
	ent := mustUserEntity(t)
	sql, args, err := And(
		ent.Col("id").In(1, 2),
		ent.Col("name").Eq("x"),
	).Render(dialect.For(config.DBTypePostgres))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := `("metadata_user"."id" IN ($1, $2) AND "metadata_user"."name" = $3)`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v, want 3 values", args)
	}
}

func TestRenderFailures(t *testing.T) {
	ent := mustUserEntity(t)
	d := dialect.For(config.DBTypeSQLite)

	if _, _, err := ent.Col("id").Eq(1).Render(nil); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Render(nil) error = %v, want ErrInvalidQuery", err)
	}

	cases := []struct {
		name string
		cond Condition
	}{
		{"unknown column", ent.Col("missing").Eq(1)},
		{"nil operand", ent.Col("name").Eq(nil)},
		{"nil column operand", ent.Col("name").Eq(Column{})},
		{"nil in In", ent.Col("id").In(1, nil)},
		{"empty And", And()},
		{"empty Or", Or()},
		{"zero Condition", Condition{}},
		{"And with nil condition", And(ent.Col("id").Eq(1), Condition{})},
		{"unknown Like mode", ent.Col("name").Like(LikeMode(99), "x")},
		{"nil entity column", Column{}.Eq(1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := tc.cond.Render(d); !errors.Is(err, errs.ErrInvalidQuery) {
				t.Errorf("Render() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
}

func TestNilEntityColumnFailsOnRender(t *testing.T) {
	var e *Entity
	cond := e.Col("id").Eq(1)
	if _, _, err := cond.Render(dialect.For(config.DBTypeSQLite)); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Errorf("Render() error = %v, want ErrInvalidQuery", err)
	}
}

func TestConditionEntities(t *testing.T) {
	user := mustUserEntity(t)
	task, err := EntityOf[defaultKey]()
	if err != nil {
		t.Fatalf("EntityOf[defaultKey]() = %v", err)
	}

	cond := And(
		user.Col("id").Eq(task.Col("account")),
		Or(user.Col("bio").IsNull(), task.Col("value").Gt(1)),
	)

	entities := cond.Entities()
	if len(entities) != 2 {
		t.Fatalf("Entities() = %v, want 2 entities", entities)
	}
	found := map[*Entity]bool{}
	for _, ent := range entities {
		found[ent] = true
	}
	if !found[user] || !found[task] {
		t.Errorf("Entities() missing expected entities: %v", entities)
	}
}
