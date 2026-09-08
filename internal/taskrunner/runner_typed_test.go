package taskrunner_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/taskrunner"
)

type scanParams struct {
	Full bool `json:"full"`
}

func TestRegisterEnqueueTypedParams(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{QueueSize: 2, Parallelism: 1})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	got := make(chan scanParams, 1)
	taskrunner.Register[scanParams](runner, func(_ context.Context, _ *slog.Logger, p scanParams) error {
		got <- p
		return nil
	}, "typed", 1)

	runner.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = runner.Shutdown(ctx)
	})

	if _, _, err := taskrunner.Enqueue(runner, "typed", scanParams{Full: true}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case p := <-got:
		if !p.Full {
			t.Fatalf("handler saw Full=false, want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("typed task did not run")
	}
}
