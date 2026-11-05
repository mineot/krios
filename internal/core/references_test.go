package core

import (
	"errors"
	"testing"

	"github.com/mineot/krios/internal/compiler"
	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expressions"
)

func TestClientOperandIdentity(t *testing.T) {
	cfg, _ := setup(t)
	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.identity == second.identity {
		t.Fatal("clients sharing pool/config share identity")
	}
	ref := first.Ref("id")
	expr := first.SumExpr(ref)
	for _, operand := range []any{ref, expr, second.CountExpr(ref)} {
		c := compiler.New(cfg.Dialect, second.identity)
		c.Operand(operand, "")
		_, _, err := c.Result()
		if !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("foreign operand accepted: %v", err)
		}
	}
	cases := []struct {
		operand any
		want    string
	}{
		{first.RefParts("u", "a.b"), `"u"."a.b"`},
		{first.CountExpr("*"), `COUNT(*)`},
		{first.MaxExpr("id"), `MAX("id")`},
		{first.MinExpr("id"), `MIN("id")`},
		{first.AvgExpr("id"), `AVG("id")`},
		{expr, `SUM("id")`},
	}
	for _, tc := range cases {
		c := compiler.New(cfg.Dialect, first.identity)
		c.Operand(tc.operand, "")
		got, _, err := c.Result()
		if err != nil || got != tc.want {
			t.Fatalf("result = %q, %v; want %q", got, err, tc.want)
		}
	}
	if ErrInvalidQuery != errs.ErrInvalidQuery || ErrInvalidConfig != errs.ErrInvalidConfig || ErrInvalidMigration != errs.ErrInvalidMigration {
		t.Fatal("error categories diverged")
	}
	var zero Client
	if _, _, err := expressions.ReadReference(zero.Ref("id"), zero.identity); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("zero client produced valid operand")
	}
}
