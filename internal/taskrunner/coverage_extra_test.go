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
	"github.com/go-bumbu/tempo/dbschedule"
	"github.com/go-bumbu/tempo/filelog"
	"github.com/go-bumbu/tempo/schedule"
	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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

func TestNewSchedulerValidation(t *testing.T) {
	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{Enqueuer: fakeEnqueuer{}}); err == nil {
		t.Fatal("expected error with nil DB")
	}
	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{DB: testDB(t)}); err == nil {
		t.Fatal("expected error with nil enqueuer")
	}
	if _, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{DB: testDB(t), Enqueuer: fakeEnqueuer{}}); err != nil {
		t.Fatalf("expected scheduler created: %v", err)
	}
}

func TestSchedulerLifecycle(t *testing.T) {
	db := testDB(t)

	// Seed a broken-cron row straight through the store, bypassing validation, so
	// Start's reload exercises its warn-and-skip branch.
	store, err := dbschedule.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), schedule.Schedule{
		ID:       uuid.New(),
		TaskName: "broken",
		Cron:     "not-a-cron",
		Enabled:  true,
	}); err != nil {
		t.Fatal(err)
	}

	sched, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{
		DB:       db,
		Enqueuer: fakeEnqueuer{},
		Logger:   discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// A valid schedule added at runtime schedules cleanly.
	if _, err := sched.Create(context.Background(), "valid", "0 0 * * * *", true, nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestTaskExecutionStore(t *testing.T) {
	db := testDB(t)
	store, err := taskrunner.NewTaskExecutionStore(db, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := uuid.New()
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
	runner.RegisterTask(func(ctx context.Context, log *slog.Logger) error { return nil }, "t", 1)
	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	_ = runner.List() // covers List

	// Canceling an unknown execution id returns an error.
	if err := runner.Cancel(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error canceling unknown id")
	}
}

func TestRunnerGetTaskLog(t *testing.T) {
	// Inject a mem sink (also a tempo.TaskLogReader) so GetTaskLog has a
	// backing store without touching disk or running a task.
	mem := tempo.NewMemTaskLogSink()
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, LogSink: mem})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := uuid.New()
	_ = mem.Append(ctx, id, "INFO", "first line")
	_ = mem.Append(ctx, id, "ERROR", "second line")

	text, err := runner.GetTaskLog(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "INFO first line") || !strings.Contains(text, "ERROR second line") {
		t.Fatalf("unexpected log text: %q", text)
	}

	// Unknown id -> empty, no error.
	empty, err := runner.GetTaskLog(ctx, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Fatalf("expected empty for unknown id, got %q", empty)
	}
}

func TestRunnerStartupSweepsOrphanLogs(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed an orphan .jsonl log for a task the recovered queue won't know about.
	orphan := uuid.New()
	orphanPath := filepath.Join(dir, orphan.String()+".jsonl")
	if err := os.WriteFile(orphanPath, []byte(`{"at":"2020-01-01T00:00:00Z","level":"INFO","msg":"stale"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A runner with DB persistence recovering zero tasks must sweep the orphan
	// at construction (tempo calls filelog.Store.RetainOnly in NewQueueRunner).
	db := testDB(t)
	if _, err := taskrunner.NewRunner(taskrunner.Cfg{
		Parallelism: 1, QueueSize: 5, DB: db, LogDir: dir, Logger: discardLogger(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("expected orphan log swept at startup, stat err = %v", err)
	}
}

func TestNewRunnerLogDirError(t *testing.T) {
	// filelog.New fails when LogDir can't be created (here: a path under an
	// existing regular file), and NewRunner surfaces that error.
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, LogDir: filepath.Join(f, "sub")}); err == nil {
		t.Fatal("expected error creating task log sink under a regular file")
	}
}

func TestRunnerGetTaskLogFilelogRoundTrip(t *testing.T) {
	// Unlike TestRunnerGetTaskLog's MemTaskLogSink, a real filelog.Store exercises
	// the on-disk JSON round-trip: local time.Now() -> JSON .jsonl -> UTC RFC3339Nano.
	store, err := filelog.New(filelog.Config{Dir: t.TempDir(), DirPerm: 0o750})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, LogSink: store})
	if err != nil {
		t.Fatal(err)
	}

	// Append only after NewRunner returns: construction runs tempo's startup
	// RetainOnly sweep over the store, and with no DB (MemPersistence recovers
	// nothing) it would delete any pre-existing files.
	ctx := context.Background()
	id := uuid.New()
	if err := store.Append(ctx, id, "INFO", "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, id, "ERROR", "boom"); err != nil {
		t.Fatal(err)
	}

	text, err := runner.GetTaskLog(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %q", len(lines), text)
	}

	wantLevels := []string{"INFO", "ERROR"}
	wantMessages := []string{"hello world", "boom"}
	for i, line := range lines {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			t.Fatalf("line %d: expected 3 space-separated fields, got %d: %q", i, len(parts), line)
		}
		if _, err := time.Parse(time.RFC3339Nano, parts[0]); err != nil {
			t.Fatalf("line %d: timestamp %q did not parse as RFC3339Nano: %v", i, parts[0], err)
		}
		if parts[1] != wantLevels[i] {
			t.Fatalf("line %d: level = %q, want %q", i, parts[1], wantLevels[i])
		}
		if parts[2] != wantMessages[i] {
			t.Fatalf("line %d: message = %q, want %q", i, parts[2], wantMessages[i])
		}
	}

	// Unknown id -> empty, no error.
	empty, err := runner.GetTaskLog(ctx, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Fatalf("expected empty for unknown id, got %q", empty)
	}
}
