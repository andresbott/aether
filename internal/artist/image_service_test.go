package artist_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andresbott/aether/internal/artist"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

type fakeFetcher struct {
	data []byte
	ext  string
}

func (f fakeFetcher) Fetch(context.Context, string) ([]byte, string, error) {
	return f.data, f.ext, nil
}

type errFetcher struct{ err error }

func (f errFetcher) Fetch(context.Context, string) ([]byte, string, error) {
	return nil, "", f.err
}

func TestFetchAndStore_Success(t *testing.T) {
	as := assetstore.New(t.TempDir())
	svc := artist.NewImageService(as, fakeFetcher{data: []byte("IMG"), ext: "jpg"})

	stored, err := svc.FetchAndStore(context.Background(), model.Artist{MBArtistID: "mbid-a", NameNorm: "a"})
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

func TestFetchAndStore_NoImageFound(t *testing.T) {
	as := assetstore.New(t.TempDir())
	svc := artist.NewImageService(as, fakeFetcher{}) // nil data -> provider found nothing

	stored, err := svc.FetchAndStore(context.Background(), model.Artist{MBArtistID: "mbid-b", NameNorm: "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored {
		t.Fatal("expected no image to be stored")
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-b"); ok {
		t.Fatal("nothing should be stored when the provider returns no image")
	}
}

func TestFetchAndStore_FetchError(t *testing.T) {
	as := assetstore.New(t.TempDir())
	svc := artist.NewImageService(as, errFetcher{err: errors.New("provider down")})

	stored, err := svc.FetchAndStore(context.Background(), model.Artist{MBArtistID: "mbid-c", NameNorm: "c"})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if stored {
		t.Fatal("expected stored=false on error")
	}
}
