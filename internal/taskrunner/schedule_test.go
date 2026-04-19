package taskrunner_test

import (
	"context"
	"testing"

	"github.com/andresbott/aether/internal/taskrunner"
)

func TestScheduleStoreCRUD(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sch, err := store.Create(ctx, taskrunner.Schedule{
		TaskName:       "scan",
		CronExpression: "0 * * * *",
		Enabled:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sch.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := store.GetByTaskName(ctx, "scan")
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpression != "0 * * * *" {
		t.Fatalf("unexpected cron: %s", got.CronExpression)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	if err := store.Delete(ctx, sch.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.GetByTaskName(ctx, "scan")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestScheduleStoreUpsert(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	s1, err := store.UpsertByTaskName(ctx, "scan", "0 * * * *", true)
	if err != nil {
		t.Fatal(err)
	}

	s2, err := store.UpsertByTaskName(ctx, "scan", "0 0 * * *", false)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ID != s1.ID {
		t.Fatal("expected same ID on upsert")
	}
	if s2.CronExpression != "0 0 * * *" || s2.Enabled {
		t.Fatalf("unexpected after upsert: %+v", s2)
	}
}
