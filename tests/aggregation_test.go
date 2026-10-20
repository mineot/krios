package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/mineot/krios"
)

type taskAggregate struct {
	OwnerID int64 `krios:"owner_id"`
	Total   int64 `krios:"total"`
	SumID   int64 `krios:"sum_id"`
	MaxID   int64 `krios:"max_id"`
	MinID   int64 `krios:"min_id"`
}

type taskOwner struct {
	OwnerID int64 `krios:"owner_id"`
}

func TestSelectAggregations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}
	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	var stats []taskAggregate
	err = db.Select(tasks).
		Fields(
			tasks.Col("owner_id"),
			krios.Count().As("total"),
			tasks.Col("id").Sum().As("sum_id"),
			tasks.Col("id").Max().As("max_id"),
			tasks.Col("id").Min().As("min_id"),
		).
		GroupBy(tasks.Col("owner_id")).
		OrderBy(tasks.Col("owner_id").Asc()).
		All(ctx, &stats)
	if err != nil {
		t.Fatalf("All() = %v", err)
	}
	want := []taskAggregate{
		{OwnerID: 1, Total: 2, SumID: 3, MaxID: 2, MinID: 1},
		{OwnerID: 2, Total: 1, SumID: 3, MaxID: 3, MinID: 3},
	}
	if len(stats) != len(want) {
		t.Fatalf("All() returned %d groups, want %d", len(stats), len(want))
	}
	for i := range want {
		if stats[i] != want[i] {
			t.Errorf("group %d = %+v, want %+v", i, stats[i], want[i])
		}
	}

	count, err := db.Select(users).Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %d, want 3", count)
	}

	count, err = db.Select(users).Where(users.Col("bio").IsNotNull()).Limit(1).Count(ctx)
	if err != nil {
		t.Fatalf("Count() with limit = %v", err)
	}
	if count != 1 {
		t.Errorf("Count() with limit = %d, want 1 (limit must be ignored)", count)
	}

	count, err = db.Select(tasks).
		Fields(tasks.Col("owner_id"), krios.Count().As("total")).
		GroupBy(tasks.Col("owner_id")).
		Count(ctx)
	if err != nil {
		t.Fatalf("Count() grouped = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() grouped = %d, want 2 groups", count)
	}
}

func TestSelectDistinctOrderLimitOffset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}
	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}

	var distinct []taskOwner
	err = db.Select(tasks).
		Fields(tasks.Col("owner_id")).
		Distinct().
		OrderBy(tasks.Col("owner_id").Desc()).
		All(ctx, &distinct)
	if err != nil {
		t.Fatalf("All() distinct = %v", err)
	}
	if len(distinct) != 2 || distinct[0].OwnerID != 2 || distinct[1].OwnerID != 1 {
		t.Errorf("distinct owners = %+v, want [2 1]", distinct)
	}

	var page []selectUser
	err = db.Select(users).
		OrderBy(users.Col("name").Asc()).
		Limit(2).
		Offset(1).
		All(ctx, &page)
	if err != nil {
		t.Fatalf("All() limit/offset = %v", err)
	}
	if len(page) != 2 || page[0].Name != "bob" || page[1].Name != "carol" {
		t.Errorf("limit/offset = %+v, want bob and carol", page)
	}
}

func TestSelectPaginate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	var first []selectTask
	page, err := db.Select(tasks).
		OrderBy(tasks.Col("id").Asc()).
		Paginate(ctx, 2, 0, &first)
	if err != nil {
		t.Fatalf("Paginate() = %v", err)
	}
	if page.Total != 3 || page.Limit != 2 || page.Offset != 0 || page.Page != 1 || page.Pages != 2 ||
		!page.HasNext || page.HasPrev {
		t.Errorf("first page = %+v", page)
	}
	if len(first) != 2 || first[0].Title != "write" || first[1].Title != "review" {
		t.Errorf("first page rows = %+v", first)
	}

	var second []selectTask
	page, err = db.Select(tasks).
		OrderBy(tasks.Col("id").Asc()).
		Paginate(ctx, 2, 2, &second)
	if err != nil {
		t.Fatalf("Paginate() second = %v", err)
	}
	if page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev {
		t.Errorf("second page = %+v", page)
	}
	if len(second) != 1 || second[0].Title != "deploy" {
		t.Errorf("second page rows = %+v", second)
	}

	var empty []selectTask
	page, err = db.Select(tasks).Paginate(ctx, 2, 10, &empty)
	if err != nil {
		t.Fatalf("Paginate() empty = %v", err)
	}
	if page.Total != 3 || page.Page != 6 || page.Pages != 2 || page.HasNext || !page.HasPrev {
		t.Errorf("empty page = %+v", page)
	}
	if len(empty) != 0 {
		t.Errorf("empty page rows = %+v, want none", empty)
	}

	// Paginate does not change the builder's own limit.
	var limited []selectTask
	_, err = db.Select(tasks).Limit(1).Paginate(ctx, 2, 0, &limited)
	if err != nil {
		t.Fatalf("Paginate() with builder limit = %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Paginate() used %d rows, want 2 (its own limit)", len(limited))
	}
	var after []selectTask
	if err := db.Select(tasks).Limit(1).All(ctx, &after); err != nil {
		t.Fatalf("All() = %v", err)
	}
	if len(after) != 1 {
		t.Errorf("builder Limit(1) returned %d rows after Paginate, want 1", len(after))
	}

	// Total respects DISTINCT.
	var owners []taskOwner
	page, err = db.Select(tasks).
		Fields(tasks.Col("owner_id")).
		Distinct().
		Paginate(ctx, 1, 0, &owners)
	if err != nil {
		t.Fatalf("Paginate() distinct = %v", err)
	}
	if page.Total != 2 || page.Pages != 2 || len(owners) != 1 {
		t.Errorf("distinct page = %+v, rows = %+v", page, owners)
	}

	// Errors are reported before execution.
	if _, err := db.Select(tasks).Paginate(ctx, 0, 0, &[]selectTask{}); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Errorf("Paginate(limit=0) = %v, want ErrInvalidQuery", err)
	}
	if _, err := db.Select(tasks).Paginate(ctx, 10, -1, &[]selectTask{}); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Errorf("Paginate(offset=-1) = %v, want ErrInvalidQuery", err)
	}
	if err := db.Select(tasks).Limit(-1).All(ctx, &[]selectTask{}); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Errorf("Limit(-1).All() = %v, want ErrInvalidQuery", err)
	}
}

func TestSelectPaginateInsideTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	defer tx.Rollback(ctx)

	var rows []selectTask
	page, err := tx.Select(tasks).OrderBy(tasks.Col("id").Asc()).Paginate(ctx, 2, 0, &rows)
	if err != nil {
		t.Fatalf("Paginate() in tx = %v", err)
	}
	if page.Total != 3 || len(rows) != 2 {
		t.Errorf("tx page = %+v, rows = %d", page, len(rows))
	}
}
