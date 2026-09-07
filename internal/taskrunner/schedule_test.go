package taskrunner_test

import (
	"context"
	"errors"
	"sync"
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

func TestSchedulerUpsertByTaskName(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()

	s1, err := s.UpsertByTaskName(ctx, "scan", "0 0 * * * *", true)
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == "" {
		t.Fatal("expected a non-empty schedule id")
	}
	if s1.CronExpression != "0 0 * * * *" || !s1.Enabled {
		t.Fatalf("unexpected schedule: %+v", s1)
	}

	// A second upsert for the same task updates in place: same id, one per task.
	s2, err := s.UpsertByTaskName(ctx, "scan", "0 0 0 * * *", false)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ID != s1.ID {
		t.Fatalf("upsert created a new schedule: %q != %q", s2.ID, s1.ID)
	}
	if s2.CronExpression != "0 0 0 * * *" || s2.Enabled {
		t.Fatalf("unexpected after upsert: %+v", s2)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(list))
	}
}

func TestSchedulerGetByTaskName(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	if _, err := s.GetByTaskName(ctx, "scan"); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("expected ErrScheduleNotFound, got %v", err)
	}
	if _, err := s.UpsertByTaskName(ctx, "scan", "0 0 0 * * *", true); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetByTaskName(ctx, "scan")
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpression != "0 0 0 * * *" {
		t.Fatalf("cron = %q", got.CronExpression)
	}
}

func TestSchedulerPatchByTaskName(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	if _, err := s.UpsertByTaskName(ctx, "scan", "0 0 0 * * *", true); err != nil {
		t.Fatal(err)
	}
	off := false
	got, err := s.PatchByTaskName(ctx, "scan", nil, &off)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatal("expected Enabled=false")
	}
	if got.CronExpression != "0 0 0 * * *" {
		t.Fatalf("cron changed: %q", got.CronExpression)
	}
	if _, err := s.PatchByTaskName(ctx, "missing", nil, &off); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("expected ErrScheduleNotFound, got %v", err)
	}
}

func TestSchedulerUpsertByTaskNameConcurrent(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = s.UpsertByTaskName(ctx, "scan", "0 0 0 * * *", true)
		}()
	}
	wg.Wait()
	list, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 schedule after concurrent upserts, got %d", len(list))
	}
}

func TestSchedulerDeleteByTaskName(t *testing.T) {
	s := newTestScheduler(t)
	ctx := context.Background()
	if _, err := s.UpsertByTaskName(ctx, "scan", "0 0 0 * * *", true); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteByTaskName(ctx, "scan"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetByTaskName(ctx, "scan"); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
	if err := s.DeleteByTaskName(ctx, "missing"); !errors.Is(err, taskrunner.ErrScheduleNotFound) {
		t.Fatalf("expected ErrScheduleNotFound, got %v", err)
	}
}
