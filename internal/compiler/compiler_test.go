package compiler_test

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mineot/krios/internal/compiler"
	"github.com/mineot/krios/internal/dialects"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expressions"
)

func TestOperandsAcrossDialects(t *testing.T) {
	owner := expressions.NewIdentity()
	cases := []struct {
		name    string
		operand any
		alias   string
		want    string
	}{
		{"simple", "id", "", `"id"`},
		{"qualified", expressions.NewReference(owner, "u.id"), "ignored", `"u"."id"`},
		{"literal dots", expressions.NewReferenceParts(owner, "u", "a.b"), "", `"u"."a.b"`},
		{"literal name alias", expressions.NewReferenceParts(owner, "a.b"), "u", `"u"."a.b"`},
		{"qualified wildcard", expressions.NewReference(owner, "u.*"), "ignored", `"u".*`},
		{"bare wildcard", "*", "u", `*`},
		{"literal star", expressions.NewReferenceParts(owner, "u", "*"), "", `"u"."*"`},
		{"literal star alias", expressions.NewReferenceParts(owner, "*"), "u", `"u"."*"`},
		{"partial star", "pre*fix", "", `"pre*fix"`},
		{"unicode spaces", " Nome ç ", "", `" Nome ç "`},
		{"literal SQL", "COUNT(id)", "", `"COUNT(id)"`},
		{"alias", "id", "u", `"u"."id"`},
		{"literal alias dots", "id", "a.b", `"a.b"."id"`},
		{"count", expressions.Count(owner, "*"), "u", `COUNT(*)`},
		{"max", expressions.Max(owner, "id"), "u", `MAX("u"."id")`},
		{"min", expressions.Min(owner, "id"), "", `MIN("id")`},
		{"avg", expressions.Avg(owner, expressions.NewReference(owner, "o.total")), "u", `AVG("o"."total")`},
		{"sum", expressions.Sum(owner, "o.total"), "", `SUM("o"."total")`},
		{"nested", expressions.Sum(owner, expressions.Max(owner, "id")), "", `SUM(MAX("id"))`},
	}
	for _, dialect := range []dialects.Dialect{dialects.PostgreSQL, dialects.MySQL, dialects.SQLite3} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				c := compiler.New(dialect, owner)
				c.Operand(tc.operand, tc.alias)
				sql, args, err := c.Result()
				want := tc.want
				if dialect == dialects.MySQL {
					want = strings.ReplaceAll(want, `"`, "`")
				}
				if err != nil || sql != want || len(args) != 0 {
					t.Fatalf("result = %q, %v, %v; want %q", sql, args, err, want)
				}
			})
		}
	}
}

func TestInjectionTextIsNotSyntax(t *testing.T) {
	owner := expressions.NewIdentity()
	for _, tc := range []struct {
		dialect     dialects.Dialect
		input, want string
	}{
		{dialects.PostgreSQL, `name"; DROP TABLE users; --`, `"name""; DROP TABLE users; --" = $1`},
		{dialects.SQLite3, `name"; DROP TABLE users; --`, `"name""; DROP TABLE users; --" = ?`},
		{dialects.MySQL, "name`; DROP TABLE users; --", "`name``; DROP TABLE users; --` = ?"},
	} {
		t.Run(string(tc.dialect), func(t *testing.T) {
			c := compiler.New(tc.dialect, owner)
			c.Operand(tc.input, "")
			c.Write(" = ")
			c.Parameter(tc.input)
			sql, args, err := c.Result()
			if err != nil || sql != tc.want || len(args) != 1 || args[0] != tc.input {
				t.Fatalf("result = %q, %v, %v", sql, args, err)
			}
		})
	}
}

type valuer struct{ calls int }

func (v *valuer) Value() (driver.Value, error) { v.calls++; return nil, errors.New("must not convert") }

func TestParametersAndIndependentCommands(t *testing.T) {
	owner := expressions.NewIdentity()
	v := &valuer{}
	number := 3
	blob := []byte{1, 2}
	now := time.Now()
	values := []any{nil, blob, now, &number, v, "secret' --", int64(7), uint64(8), false, []int{9, 10}}
	for _, dialect := range []dialects.Dialect{dialects.PostgreSQL, dialects.SQLite3, dialects.MySQL} {
		t.Run(string(dialect), func(t *testing.T) {
			c := compiler.New(dialect, owner)
			fragment := func(start, end int) {
				for i := start; i < end; i++ {
					if i > 0 {
						c.Write(", ")
					}
					c.Parameter(values[i])
				}
			}
			fragment(0, 4)
			fragment(4, len(values))
			sql, args, err := c.Result()
			want := strings.TrimSuffix(strings.Repeat("?, ", 10), ", ")
			if dialect == dialects.PostgreSQL {
				want = "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10"
			}
			if err != nil || sql != want || !reflect.DeepEqual(args, values) || v.calls != 0 {
				t.Fatalf("incorrect parameters: %q, %v", sql, err)
			}
			if args[3] != &number || args[4] != v || &args[1].([]byte)[0] != &blob[0] {
				t.Fatal("argument identity changed")
			}
			args[0] = "changed"
			_, fresh, _ := c.Result()
			if fresh[0] != nil {
				t.Fatal("result exposed argument slice")
			}
			next := compiler.New(dialect, owner)
			next.Parameter("fresh")
			nextSQL, nextArgs, err := next.Result()
			first := "?"
			if dialect == dialects.PostgreSQL {
				first = "$1"
			}
			if err != nil || nextSQL != first || !reflect.DeepEqual(nextArgs, []any{"fresh"}) {
				t.Fatal("command state reused")
			}
		})
	}
}

func TestInvalidOperandsDiscardPartialResult(t *testing.T) {
	owner := expressions.NewIdentity()
	other := expressions.NewIdentity()
	invalid := []any{nil, 42, []string{"id"}, "", "a..b", "a\x00b", "*.id", (*expressions.Reference)(nil), (*expressions.Expression)(nil), &expressions.Reference{}, &expressions.Expression{}, expressions.NewReferenceParts(owner), expressions.NewReference(other, "secret"), expressions.Count(other, "id"), expressions.Sum(owner, expressions.Count(other, "id")), expressions.Sum(owner, ""), expressions.Sum(owner, 123)}
	for i, operand := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := compiler.New(dialects.PostgreSQL, owner)
			c.Write("SELECT ")
			c.Parameter("secret")
			c.Operand(operand, "")
			sql, args, err := c.Result()
			if sql != "" || args != nil || !errors.Is(err, errs.ErrInvalidQuery) {
				t.Fatalf("partial result leaked: %q, %v, %v", sql, args, err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("sensitive operand leaked")
			}
			c.Write("anything")
			c.Parameter(1)
			c.Operand("valid", "")
			_, _, again := c.Result()
			if again != err {
				t.Fatal("error was not sticky")
			}
		})
	}
	for _, c := range []*compiler.Compiler{compiler.New("invalid", owner), compiler.New(dialects.MySQL, nil), compiler.New(dialects.MySQL, &expressions.Identity{})} {
		c.Parameter(1)
		_, _, err := c.Result()
		if !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("invalid compiler accepted: %v", err)
		}
	}
	c := compiler.New(dialects.PostgreSQL, owner)
	c.Operand("id", "bad\x00alias")
	_, _, err := c.Result()
	if !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatal("invalid alias accepted")
	}
}

func TestConcurrentIndependentCompilation(t *testing.T) {
	owner := expressions.NewIdentity()
	ref := expressions.NewReferenceParts(owner, "o", "price.with.dot")
	expr := expressions.Sum(owner, ref)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				c := compiler.New(dialects.PostgreSQL, owner)
				c.Operand(expr, "")
				c.Write(" > ")
				c.Parameter(j)
				sql, args, err := c.Result()
				if err != nil || sql != `SUM("o"."price.with.dot") > $1` || len(args) != 1 || args[0] != j {
					t.Errorf("shared operand state changed: %q, %v", sql, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestExpressionCycle(t *testing.T) {
	owner := expressions.NewIdentity()
	expr := expressions.Count(owner, "id")
	// Assignment to an exported struct can create a cycle despite private fields.
	*expr = *expressions.Sum(owner, expr)
	c := compiler.New(dialects.PostgreSQL, owner)
	c.Operand(expr, "")
	sql, args, err := c.Result()
	if sql != "" || args != nil || !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatalf("cycle result = %q, %v, %v", sql, args, err)
	}
}
