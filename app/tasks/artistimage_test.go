package tasks

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/tempo"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

type fakeFetcher struct {
	calls int
	data  []byte
	ext   string
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	f.calls++
	return f.data, f.ext, nil
}

// TestFetchTaskNotConfigured verifies that when no provider is configured
// (nil fetcher), the task returns a clear, actionable "not configured" error
// (which the runner records in the per-execution log) instead of silently
// doing nothing.
func TestFetchTaskNotConfigured(t *testing.T) {
	st := newTestStore(t)
	as := assetstore.New(t.TempDir())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	fn := NewFetchArtistImagesTaskFn(st, as, nil, logger, time.Hour)
	err := fn(context.Background())
	if err == nil {
		t.Fatal("expected a 'not configured' error, got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected a 'not configured' error, got: %v", err)
	}
}

type errFetcher struct{ err error }

func (f *errFetcher) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", f.err
}

// TestFetchTaskLogsFetchErrors verifies a provider fetch failure is written to
// the task's per-execution log (not silently swallowed). The task is run through
// the runner so the tempo task-scoped logger is present in the context.
func TestFetchTaskLogsFetchErrors(t *testing.T) {
	st := newTestStore(t)
	_ = st.Transaction(func(tx *store.Store) error {
		_, e := tx.FindOrCreateArtists([]string{"Pink Floyd"}, []string{"mbid-pf"})
		return e
	})
	as := assetstore.New(t.TempDir())
	f := &errFetcher{err: errors.New("provider down")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	sink := tempo.NewMemTaskLogSink()
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{Parallelism: 1, QueueSize: 5, LogSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	runner.RegisterTask(NewFetchArtistImagesTaskFn(st, as, f, logger, time.Hour), "fetch", 1)
	runner.Start()
	defer func() { _ = runner.Shutdown(context.Background()) }()

	id, err := runner.AddRun("fetch")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	entries := sink.Logs(id)
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, "provider down") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected the fetch error to be logged to the per-execution log; got %d entries: %+v", len(entries), entries)
	}
}

func TestFetchTaskStoresImageAndSkipsExisting(t *testing.T) {
	st := newTestStore(t)
	_ = st.Transaction(func(tx *store.Store) error {
		_, e := tx.FindOrCreateArtists([]string{"A"}, []string{"mbid-a"})
		return e
	})
	as := assetstore.New(t.TempDir())
	f := &fakeFetcher{data: []byte("IMG"), ext: "jpg"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	fn := NewFetchArtistImagesTaskFn(st, as, f, logger, time.Hour)
	if err := fn(context.Background()); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-a"); !ok {
		t.Fatal("image not stored")
	}
	if f.calls != 1 {
		t.Fatalf("expected 1 fetch, got %d", f.calls)
	}
	// Second run: image exists, must skip the fetcher.
	if err := fn(context.Background()); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("expected no extra fetch, got %d", f.calls)
	}
}

func TestFetchAndStoreArtistImage_Success(t *testing.T) {
	st := newTestStore(t)
	var artist *model.Artist
	_ = st.Transaction(func(tx *store.Store) error {
		artists, e := tx.FindOrCreateArtists([]string{"A"}, []string{"mbid-a"})
		artist = artists[0]
		return e
	})
	as := assetstore.New(t.TempDir())
	f := &fakeFetcher{data: []byte("IMG"), ext: "jpg"}

	stored, err := FetchAndStoreArtistImage(context.Background(), st, as, f, *artist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stored {
		t.Fatal("expected image to be stored")
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-a"); !ok {
		t.Fatal("image not persisted in asset store")
	}
}

func TestFetchAndStoreArtistImage_NoImageFound(t *testing.T) {
	st := newTestStore(t)
	var artist *model.Artist
	_ = st.Transaction(func(tx *store.Store) error {
		artists, e := tx.FindOrCreateArtists([]string{"B"}, []string{"mbid-b"})
		artist = artists[0]
		return e
	})
	as := assetstore.New(t.TempDir())
	f := &fakeFetcher{} // nil data -> provider found nothing

	stored, err := FetchAndStoreArtistImage(context.Background(), st, as, f, *artist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored {
		t.Fatal("expected no image to be stored")
	}
}

func TestFetchAndStoreArtistImage_FetchError(t *testing.T) {
	st := newTestStore(t)
	var artist *model.Artist
	_ = st.Transaction(func(tx *store.Store) error {
		artists, e := tx.FindOrCreateArtists([]string{"C"}, []string{"mbid-c"})
		artist = artists[0]
		return e
	})
	as := assetstore.New(t.TempDir())
	f := &errFetcher{err: errors.New("provider down")}

	stored, err := FetchAndStoreArtistImage(context.Background(), st, as, f, *artist)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if stored {
		t.Fatal("expected stored=false on error")
	}
}
