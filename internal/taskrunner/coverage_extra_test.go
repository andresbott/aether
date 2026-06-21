package taskrunner_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/go-bumbu/tempo"
	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFileTaskLogSinkAndReader(t *testing.T) {
	dir := t.TempDir()
	sink, err := taskrunner.NewFileTaskLogSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := uuid.New()
	if err := sink.Append(ctx, id, "INFO", "first line"); err != nil {
		t.Fatal(err)
	}
	if err := sink.Append(ctx, id, "ERROR", "second line"); err != nil {
		t.Fatal(err)
	}

	reader := taskrunner.NewFileTaskLogReader(dir)
	text, err := reader.GetTaskLog(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "first line") || !strings.Contains(text, "second line") {
		t.Fatalf("unexpected log content: %q", text)
	}

	// Unknown id -> empty string, no error.
	empty, err := reader.GetTaskLog(ctx, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Fatalf("expected empty log for unknown id, got %q", empty)
	}

	// Remove logs, then the reader returns empty again.
	if err := sink.RemoveTaskLogs(ctx, []uuid.UUID{id}); err != nil {
		t.Fatal(err)
	}
	after, err := reader.GetTaskLog(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if after != "" {
		t.Fatalf("expected empty after remove, got %q", after)
	}
	// Removing a missing file is a no-op.
	if err := sink.RemoveTaskLogs(ctx, []uuid.UUID{id}); err != nil {
		t.Fatalf("removing missing log should be a no-op: %v", err)
	}
}

func TestNewFileTaskLogSinkError(t *testing.T) {
	// Creating the sink dir under an existing regular file must fail.
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := taskrunner.NewFileTaskLogSink(filepath.Join(f, "sub")); err == nil {
		t.Fatal("expected error creating sink dir under a regular file")
	}
}

func TestScheduleStoreExtraMethods(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Both are created enabled (the dbSchedule.Enabled column defaults to true,
	// so Create can't store a false zero-value); we disable "b" via Update,
	// whose map-based write does persist false.
	a, err := store.Create(ctx, taskrunner.Schedule{TaskName: "a", CronExpression: "0 * * * *", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Create(ctx, taskrunner.Schedule{TaskName: "b", CronExpression: "0 0 * * *", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	b.Enabled = false
	if err := store.Update(ctx, b); err != nil {
		t.Fatal(err)
	}

	enabled, err := store.ListEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 1 || enabled[0].TaskName != "a" {
		t.Fatalf("ListEnabled = %v", enabled)
	}

	got, err := store.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskName != "a" {
		t.Fatalf("GetByID = %+v", got)
	}
	if _, err := store.GetByID(ctx, 99999); err == nil {
		t.Fatal("expected not found for missing id")
	}

	a.Enabled = false
	if err := store.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	updated, err := store.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled {
		t.Fatal("expected disabled after update")
	}
	if err := store.Update(ctx, taskrunner.Schedule{ID: 0}); err == nil {
		t.Fatal("expected error updating with zero id")
	}

	if err := store.DeleteByTaskName(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteByTaskName(ctx, "missing"); err == nil {
		t.Fatal("expected ErrScheduleNotFound for missing task name")
	}
}

func TestCronExpressionHelpers(t *testing.T) {
	if got := taskrunner.NormalizeCronExpression("* * * * *"); got != "0 * * * * *" {
		t.Fatalf("normalize 5-field: got %q", got)
	}
	if got := taskrunner.NormalizeCronExpression("0 * * * * *"); got != "0 * * * * *" {
		t.Fatalf("normalize 6-field unchanged: got %q", got)
	}
	if err := taskrunner.ValidateCronExpression(""); err == nil {
		t.Fatal("expected error for empty cron")
	}
	if err := taskrunner.ValidateCronExpression("0 * * * *"); err != nil {
		t.Fatalf("expected valid 5-field cron: %v", err)
	}
	if err := taskrunner.ValidateCronExpression("nonsense"); err == nil {
		t.Fatal("expected error for invalid cron")
	}
}

func TestFuncEnqueuer(t *testing.T) {
	var got string
	var f taskrunner.FuncEnqueuer = func(ctx context.Context, name string) error {
		got = name
		return nil
	}
	if err := f.EnqueueTask(context.Background(), "scan"); err != nil {
		t.Fatal(err)
	}
	if got != "scan" {
		t.Fatalf("EnqueueTask passed %q", got)
	}
}

func TestNewSchedulerValidation(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	enq := taskrunner.FuncEnqueuer(func(ctx context.Context, name string) error { return nil })

	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{Enqueuer: enq}); err == nil {
		t.Fatal("expected error with nil schedule store")
	}
	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{ScheduleStore: store}); err == nil {
		t.Fatal("expected error with nil enqueuer")
	}
	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{ScheduleStore: store, Enqueuer: enq}); err != nil {
		t.Fatalf("expected scheduler created: %v", err)
	}
}

func TestSchedulerLifecycle(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// One valid + one broken cron: exercises both the schedule and the skip branch.
	if _, err := store.Create(ctx, taskrunner.Schedule{TaskName: "valid", CronExpression: "0 * * * *", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, taskrunner.Schedule{TaskName: "broken", CronExpression: "not-a-cron", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	enq := taskrunner.FuncEnqueuer(func(ctx context.Context, name string) error { return nil })
	sched, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{
		ScheduleStore: store,
		Enqueuer:      enq,
		Logger:        discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(runCtx) // covers Start + loadSchedules (valid + skip branches)
	if err := sched.Refresh(runCtx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	sched.Stop()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	sched.Wait(waitCtx)
}

func TestTaskExecutionStore(t *testing.T) {
	db := testDB(t)
	sink, err := taskrunner.NewFileTaskLogSink(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := taskrunner.NewTaskExecutionStore(db, discardLogger(), sink)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := uuid.New()
	if err := sink.Append(ctx, id, "INFO", "log line"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTask(ctx, tempo.TaskInfo{
		ID:       id,
		Name:     "scan",
		Status:   tempo.TaskStatusRunning,
		QueuedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "scan" {
		t.Fatalf("List = %v", list)
	}

	// Empty id slice is a no-op.
	if err := store.RemoveTasks(ctx, nil); err != nil {
		t.Fatal(err)
	}
	// Removing also clears the associated log via the cleaner.
	if err := store.RemoveTasks(ctx, []uuid.UUID{id}); err != nil {
		t.Fatal(err)
	}
	list2, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list2) != 0 {
		t.Fatalf("expected empty after RemoveTasks, got %v", list2)
	}
}

func TestRunnerListAndCancel(t *testing.T) {
	db := testDB(t)
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, DB: db})
	if err != nil {
		t.Fatal(err)
	}
	runner.RegisterTask(func(ctx context.Context) error { return nil }, "t", 1)
	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	_ = runner.List() // covers List

	// Canceling an unknown execution id returns an error.
	if err := runner.Cancel(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error canceling unknown id")
	}
}
