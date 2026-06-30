package tasks

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
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
