package expressions_test

import (
	"errors"
	"testing"

	"github.com/mineot/krios/internal/errs"
	"github.com/mineot/krios/internal/expressions"
)

func TestReferenceSnapshots(t *testing.T) {
	owner := expressions.NewIdentity()
	parts := []string{"u", "name.with.dot", "*"}
	ref := expressions.NewReferenceParts(owner, parts...)
	parts[0] = "changed"
	first, wildcard, err := expressions.ReadReference(ref, owner)
	if err != nil || wildcard || first[0] != "u" || first[1] != "name.with.dot" || first[2] != "*" {
		t.Fatalf("snapshot = %v, %v, %v", first, wildcard, err)
	}
	first[1] = "mutated"
	second, _, err := expressions.ReadReference(ref, owner)
	if err != nil || second[1] != "name.with.dot" {
		t.Fatal("read exposed mutable state")
	}
}
func TestInvalidReferences(t *testing.T) {
	owner := expressions.NewIdentity()
	refs := []*expressions.Reference{nil, {}, expressions.NewReference(nil, "id"), expressions.NewReference(&expressions.Identity{}, "id"), expressions.NewReference(owner, ""), expressions.NewReference(owner, "u..id"), expressions.NewReference(owner, ".id"), expressions.NewReference(owner, "u."), expressions.NewReference(owner, "*.id"), expressions.NewReference(owner, "*.*"), expressions.NewReference(owner, "a\x00b"), expressions.NewReferenceParts(owner), expressions.NewReferenceParts(owner, "u", "")}
	for i, ref := range refs {
		if _, _, err := expressions.ReadReference(ref, owner); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("reference %d: %v", i, err)
		}
	}
}
func TestAggregateValidation(t *testing.T) {
	owner := expressions.NewIdentity()
	other := expressions.NewIdentity()
	invalid := []any{nil, 1, []string{"id"}, expressions.Reference{}, expressions.Expression{}, (*expressions.Reference)(nil), (*expressions.Expression)(nil), &expressions.Reference{}, &expressions.Expression{}, expressions.NewReference(owner, ""), expressions.NewReference(other, "id"), expressions.Count(other, "id")}
	for i, operand := range invalid {
		e := expressions.Sum(owner, operand)
		if _, _, err := expressions.ReadExpression(e, owner); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("operand %d: %v", i, err)
		}
	}
	for _, e := range []*expressions.Expression{nil, {}, expressions.Count(nil, "id"), expressions.Count(&expressions.Identity{}, "id")} {
		if _, _, err := expressions.ReadExpression(e, owner); !errors.Is(err, errs.ErrInvalidQuery) {
			t.Fatalf("invalid expression: %v", err)
		}
	}
	ref := expressions.NewReference(owner, "id")
	inner := expressions.Min(owner, ref)
	outer := expressions.Max(owner, inner)
	name, child, err := expressions.ReadExpression(outer, owner)
	if err != nil || name != "MAX" || child != inner {
		t.Fatalf("nested expression = %s, %v", name, err)
	}
	_, child, err = expressions.ReadExpression(inner, owner)
	if err != nil || child != ref {
		t.Fatal("operand was not preserved by reference")
	}
	if _, _, err := expressions.ReadExpression(outer, other); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatal("cross-client aggregate accepted")
	}
	if _, _, err := expressions.ReadReference(ref, other); !errors.Is(err, errs.ErrInvalidQuery) {
		t.Fatal("cross-client reference accepted")
	}
}
