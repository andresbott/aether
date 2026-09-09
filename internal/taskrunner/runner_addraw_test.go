package taskrunner_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/google/uuid"
)

func TestRunnerAddRaw(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{QueueSize: 2})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	runner.RegisterTask(func(context.Context, *slog.Logger) error { return nil }, "t", 1)

	// Not started, so the run stays queued; AddRaw still returns its id.
	id, reused, err := runner.AddRaw("t", nil)
	if err != nil {
		t.Fatalf("AddRaw: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected a non-nil execution id")
	}
	if reused {
		t.Fatal("first enqueue must report reused=false")
	}
}
