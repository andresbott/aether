package tasks

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/andresbott/aether/internal/scanner"
)

func TestScanCallsAfterHook(t *testing.T) {
	called := false
	st := newTestStore(t) // reuse helper
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fn := NewScanTaskFn(scanner.Config{}, st, nil, logger, false, func() { called = true })
	_ = fn(context.Background()) // scan over an empty/library-less store is fine
	if !called {
		t.Fatal("afterScan hook was not called")
	}
}
