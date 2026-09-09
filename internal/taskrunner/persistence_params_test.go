package taskrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/go-bumbu/tempo"
	"github.com/google/uuid"
)

func TestTaskExecutionStoreRoundTripsParams(t *testing.T) {
	store, err := taskrunner.NewTaskExecutionStore(testDB(t), discardLogger())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()
	want := []byte(`{"full":true}`)

	if err := store.SaveTask(ctx, tempo.TaskInfo{
		ID:       uuid.New(),
		Name:     "scan",
		Status:   tempo.TaskStatusWaiting,
		QueuedAt: time.Now(),
		Params:   want,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d rows, want 1", len(list))
	}
	if string(list[0].Params) != string(want) {
		t.Fatalf("params = %q, want %q", list[0].Params, want)
	}
}
