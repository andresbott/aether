package taskrunner_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/google/uuid"
)

// fakeEnqueuer satisfies schedule.Enqueuer for scheduler tests that never fire.
type fakeEnqueuer struct{}

func (fakeEnqueuer) AddRaw(string, []byte) (uuid.UUID, bool, error) {
	return uuid.New(), false, nil
}

// newTestScheduler builds a started scheduler on an in-memory DB, stopped on
// cleanup. testDB and discardLogger are defined elsewhere in this test package.
func newTestScheduler(t *testing.T) *taskrunner.Scheduler {
	t.Helper()
	sched, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{
		DB:       testDB(t),
		Enqueuer: fakeEnqueuer{},
		Logger:   discardLogger(),
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sched.Stop(ctx)
	})
	return sched
}

func TestSchedulerCreateListByTaskName(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()

	fast, err := s.Create(ctx, "scan", "0 0 * * * *", true, []byte(`{"full":false}`))
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.Create(ctx, "scan", "0 0 3 * * *", true, []byte(`{"full":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if fast.ID == full.ID || fast.ID == "" {
		t.Fatalf("expected two distinct non-empty ids, got %q and %q", fast.ID, full.ID)
	}

	list, err := s.ListByTaskName(ctx, "scan")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 schedules for scan, got %d", len(list))
	}
	if none, _ := s.ListByTaskName(ctx, "other"); len(none) != 0 {
		t.Fatalf("expected 0 schedules for other task, got %d", len(none))
	}
}

func TestSchedulerUpdatePartial(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	sc, err := s.Create(ctx, "scan", "0 0 3 * * *", true, []byte(`{"full":false}`))
	if err != nil {
		t.Fatal(err)
	}
	off := false
	upd, err := s.Update(ctx, sc.ID, nil, &off, []byte(`{"full":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if upd.Enabled {
		t.Fatal("expected Enabled=false")
	}
	if upd.CronExpression != sc.CronExpression {
		t.Fatalf("cron changed: %q -> %q", sc.CronExpression, upd.CronExpression)
	}
	if string(upd.Params) != `{"full":true}` {
		t.Fatalf("params = %q, want {\"full\":true}", upd.Params)
	}
}

func TestSchedulerDeleteByID(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	sc, err := s.Create(ctx, "scan", "0 0 3 * * *", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, sc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, sc.ID); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("expected ErrScheduleNotFound after delete, got %v", err)
	}
	if err := s.Delete(ctx, uuid.NewString()); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("delete missing: expected ErrScheduleNotFound, got %v", err)
	}
}
