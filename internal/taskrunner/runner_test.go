package taskrunner_test

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRunnerExecuteTask(t *testing.T) {
	db := testDB(t)
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{
		Parallelism: 1,
		QueueSize:   5,
		DB:          db,
	})
	if err != nil {
		t.Fatal(err)
	}

	var ran atomic.Bool
	runner.RegisterTask(func(ctx context.Context, log *slog.Logger) error {
		ran.Store(true)
		return nil
	}, "test-task", 1)

	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	id, _, err := runner.AddRun("test-task")
	if err != nil {
		t.Fatal(err)
	}
	if id.String() == "" {
		t.Fatal("expected non-empty ID")
	}

	time.Sleep(200 * time.Millisecond)
	if !ran.Load() {
		t.Fatal("task did not run")
	}
}

func TestRunnerExecutions(t *testing.T) {
	db := testDB(t)
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{
		Parallelism: 1,
		QueueSize:   5,
		DB:          db,
	})
	if err != nil {
		t.Fatal(err)
	}

	runner.RegisterTask(func(ctx context.Context, log *slog.Logger) error { return nil }, "test-task", 1)
	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	_, _, _ = runner.AddRun("test-task")
	time.Sleep(200 * time.Millisecond)

	execs := runner.Executions(context.Background())
	if len(execs) == 0 {
		t.Fatal("expected at least 1 execution")
	}
	if execs[0].TaskName != "test-task" {
		t.Fatalf("expected task name test-task, got %s", execs[0].TaskName)
	}
	if execs[0].Status != "complete" {
		t.Fatalf("expected status complete, got %s", execs[0].Status)
	}
}

func TestRunnerExecutionsProgress(t *testing.T) {
	db := testDB(t)
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, DB: db})
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	runner.RegisterWithProgress(func(_ context.Context, _ *slog.Logger, prog taskrunner.Progress) error {
		prog.SetTotal(10)
		prog.Inc(4)
		prog.SetStage("Extracting metadata: a/b.mp3")
		close(started)
		<-release
		return nil
	}, "prog-task", 1)

	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	id, _, _ := runner.AddRun("prog-task")
	<-started

	// The reporter flushes synchronously before started closes, but poll briefly
	// to absorb the status→running transition landing in the queue.
	var got *taskrunner.ExecutionProgress
	for i := 0; i < 50 && got == nil; i++ {
		for _, e := range runner.Executions(context.Background()) {
			if e.ID == id && e.Status == "running" && e.Progress != nil {
				got = e.Progress
			}
		}
		if got == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	close(release)

	if got == nil {
		t.Fatal("expected progress on the running execution")
	}
	if got.Total != 10 || got.Done != 4 {
		t.Fatalf("got done=%d total=%d, want 4/10", got.Done, got.Total)
	}
	if got.Stage != "Extracting metadata: a/b.mp3" {
		t.Fatalf("stage = %q", got.Stage)
	}

	// Once finished, progress is gated off (contract: present only while running).
	time.Sleep(100 * time.Millisecond)
	for _, e := range runner.Executions(context.Background()) {
		if e.ID == id && e.Progress != nil {
			t.Fatalf("finished execution still carries progress: %+v", e.Progress)
		}
	}
}

func TestRunnerSingletonCoalesces(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{})
	if err != nil {
		t.Fatal(err)
	}
	noop := func(ctx context.Context, log *slog.Logger) error { return nil }
	runner.RegisterTask(noop, "singleton-task", 1, taskrunner.Singleton())

	// The runner is never Started, so the first run stays queued: a second
	// trigger of a singleton task must coalesce onto it rather than pile up a
	// duplicate.
	id1, reused1, err := runner.AddRun("singleton-task")
	if err != nil {
		t.Fatal(err)
	}
	if reused1 {
		t.Fatal("first enqueue of a singleton task must not report reused")
	}

	id2, reused2, err := runner.AddRun("singleton-task")
	if err != nil {
		t.Fatal(err)
	}
	if !reused2 {
		t.Fatal("second enqueue of a singleton task must coalesce (reused=true)")
	}
	if id2 != id1 {
		t.Fatalf("coalesced enqueue returned id %s, want the in-flight id %s", id2, id1)
	}
}

func TestRunnerNonSingletonDoesNotCoalesce(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{})
	if err != nil {
		t.Fatal(err)
	}
	noop := func(ctx context.Context, log *slog.Logger) error { return nil }
	runner.RegisterTask(noop, "plain-task", 1)

	id1, reused1, err := runner.AddRun("plain-task")
	if err != nil {
		t.Fatal(err)
	}
	id2, reused2, err := runner.AddRun("plain-task")
	if err != nil {
		t.Fatal(err)
	}
	if reused1 || reused2 {
		t.Fatal("a non-singleton task must never report reused")
	}
	if id1 == id2 {
		t.Fatal("non-singleton enqueues must get distinct execution ids")
	}
}
