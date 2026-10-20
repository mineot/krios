package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/mineot/krios"
)

type userTaskCount struct {
	ID    int64  `krios:"id"`
	Name  string `krios:"name"`
	Tasks int64  `krios:"tasks"`
}

func TestValidationJoinGroupAggregatePagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	users, err := krios.EntityOf[selectUser]()
	if err != nil {
		t.Fatalf("EntityOf[selectUser]() = %v", err)
	}
	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	base := db.Select(users).
		Join(tasks, krios.LeftJoin, tasks.Col("owner_id").Eq(users.Col("id"))).
		Fields(
			users.Col("id"),
			users.Col("name"),
			tasks.Col("id").Count().As("tasks"),
		).
		GroupBy(users.Col("id"), users.Col("name")).
		OrderBy(tasks.Col("id").Count().Desc(), users.Col("name").Asc())

	count, err := base.Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %d, want 3 users", count)
	}

	var first []userTaskCount
	page, err := base.Paginate(ctx, 2, 0, &first)
	if err != nil {
		t.Fatalf("Paginate() first = %v", err)
	}
	if page.Total != 3 || page.Pages != 2 || page.Page != 1 || !page.HasNext || page.HasPrev {
		t.Errorf("first page = %+v", page)
	}
	wantFirst := []userTaskCount{
		{ID: 1, Name: "alice", Tasks: 2},
		{ID: 2, Name: "bob", Tasks: 1},
	}
	if len(first) != len(wantFirst) {
		t.Fatalf("first page rows = %+v, want %+v", first, wantFirst)
	}
	for i := range wantFirst {
		if first[i] != wantFirst[i] {
			t.Errorf("first page row %d = %+v, want %+v", i, first[i], wantFirst[i])
		}
	}

	var second []userTaskCount
	page, err = base.Paginate(ctx, 2, 2, &second)
	if err != nil {
		t.Fatalf("Paginate() second = %v", err)
	}
	if page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev {
		t.Errorf("second page = %+v", page)
	}
	wantSecond := []userTaskCount{{ID: 3, Name: "carol", Tasks: 0}}
	if len(second) != 1 || second[0] != wantSecond[0] {
		t.Errorf("second page rows = %+v, want %+v", second, wantSecond)
	}

	// Paginate does not leave its own limit/offset on the shared builder.
	var all []userTaskCount
	if err := base.All(ctx, &all); err != nil {
		t.Fatalf("All() after Paginate = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("All() after Paginate returned %d groups, want 3", len(all))
	}
}

func TestValidationFilteredPagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	base := db.Select(tasks).
		Where(
			tasks.Col("owner_id").Eq(int64(1)),
			tasks.Col("title").Like(krios.Contains, "e"),
		).
		OrderBy(tasks.Col("id").Asc())

	count, err := base.Count(ctx)
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}

	var pageRows []selectTask
	page, err := base.Paginate(ctx, 1, 1, &pageRows)
	if err != nil {
		t.Fatalf("Paginate() = %v", err)
	}
	if page.Total != 2 || page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev {
		t.Errorf("filtered page = %+v", page)
	}
	if len(pageRows) != 1 || pageRows[0].Title != "review" {
		t.Errorf("filtered page rows = %+v, want review", pageRows)
	}
}

func TestValidationPaginationEdges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := seedSelectDB(t)

	tasks, err := krios.EntityOf[selectTask]()
	if err != nil {
		t.Fatalf("EntityOf[selectTask]() = %v", err)
	}

	// Empty result: total and pages are zero, page is 1, destination cleared.
	empty := []selectTask{{ID: 99}}
	page, err := db.Select(tasks).
		Where(tasks.Col("title").Eq("missing")).
		Paginate(ctx, 3, 0, &empty)
	if err != nil {
		t.Fatalf("Paginate() empty = %v", err)
	}
	if page.Total != 0 || page.Pages != 0 || page.Page != 1 || page.HasNext || page.HasPrev {
		t.Errorf("empty page = %+v", page)
	}
	if len(empty) != 0 {
		t.Errorf("empty page rows = %+v, want none", empty)
	}

	// limit equal to total: single full page.
	var full []selectTask
	page, err = db.Select(tasks).Paginate(ctx, 3, 0, &full)
	if err != nil {
		t.Fatalf("Paginate() full = %v", err)
	}
	if page.Page != 1 || page.Pages != 1 || page.HasNext || page.HasPrev || len(full) != 3 {
		t.Errorf("full page = %+v, rows = %d", page, len(full))
	}

	// offset exactly at total: page 2 of 2 with no rows.
	var atEnd []selectTask
	page, err = db.Select(tasks).Paginate(ctx, 2, 3, &atEnd)
	if err != nil {
		t.Fatalf("Paginate() at end = %v", err)
	}
	if page.Page != 2 || page.Pages != 2 || page.HasNext || !page.HasPrev || len(atEnd) != 0 {
		t.Errorf("at-end page = %+v, rows = %d", page, len(atEnd))
	}

	// offset beyond total: page keeps the formula, no next rows.
	var beyond []selectTask
	page, err = db.Select(tasks).Paginate(ctx, 2, 4, &beyond)
	if err != nil {
		t.Fatalf("Paginate() beyond = %v", err)
	}
	if page.Page != 3 || page.Pages != 2 || page.HasNext || !page.HasPrev || len(beyond) != 0 {
		t.Errorf("beyond page = %+v, rows = %d", page, len(beyond))
	}

	// Limit(0) returns no rows; Count ignores it.
	var none []selectTask
	limited := db.Select(tasks).OrderBy(tasks.Col("id").Asc()).Limit(0)
	if err := limited.All(ctx, &none); err != nil {
		t.Fatalf("All() limit 0 = %v", err)
	}
	if len(none) != 0 {
		t.Errorf("Limit(0) returned %d rows, want none", len(none))
	}
	count, err := limited.Count(ctx)
	if err != nil {
		t.Fatalf("Count() limit 0 = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() with Limit(0) = %d, want 3", count)
	}
}

func TestValidationTransactionConsistency(t *testing.T) {
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
	if _, err := tx.ExecContext(ctx, "INSERT INTO tasks (id, owner_id, title) VALUES (4, 1, 'ship')"); err != nil {
		t.Fatalf("INSERT in tx = %v", err)
	}

	count, err := tx.Select(tasks).Count(ctx)
	if err != nil {
		t.Fatalf("Count() in tx = %v", err)
	}
	if count != 4 {
		t.Errorf("Count() in tx = %d, want 4 (uncommitted row must be visible)", count)
	}

	var pageRows []selectTask
	page, err := tx.Select(tasks).OrderBy(tasks.Col("id").Desc()).Paginate(ctx, 2, 0, &pageRows)
	if err != nil {
		t.Fatalf("Paginate() in tx = %v", err)
	}
	if page.Total != 4 || page.Pages != 2 || len(pageRows) != 2 || pageRows[0].Title != "ship" {
		t.Errorf("tx page = %+v, rows = %+v", page, pageRows)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() = %v", err)
	}
	count, err = db.Select(tasks).Count(ctx)
	if err != nil {
		t.Fatalf("Count() after rollback = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() after rollback = %d, want 3", count)
	}

	// A validation failure happens before execution and does not block Commit.
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() second = %v", err)
	}
	if _, err := tx.Select(tasks).Paginate(ctx, 0, 0, &[]selectTask{}); !errors.Is(err, krios.ErrInvalidQuery) {
		t.Fatalf("Paginate(limit=0) in tx = %v, want ErrInvalidQuery", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Errorf("Commit() after invalid query = %v", err)
	}
}
