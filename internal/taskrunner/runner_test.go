package taskrunner_test

import (
	"context"
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
	runner.RegisterTask(func(ctx context.Context) error {
		ran.Store(true)
		return nil
	}, "test-task", 1)

	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	id, err := runner.AddRun("test-task")
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

	runner.RegisterTask(func(ctx context.Context) error { return nil }, "test-task", 1)
	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	_, _ = runner.AddRun("test-task")
	time.Sleep(200 * time.Millisecond)

	execs := runner.Executions()
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
