package tasks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestLiveAssetKeysCoversAllKindsAndBothArtistSlots(t *testing.T) {
	ids := store.AssetIdentities{
		Albums:    []model.Album{{NameNorm: "al", AlbumArtistNorm: "ar", MBReleaseID: "rel"}},
		Artists:   []model.Artist{{NameNorm: "queen", MBArtistID: "mbid-q"}, {NameNorm: "noid"}},
		Genres:    []model.Genre{{Name: "Rock"}},
		Playlists: []model.Playlist{{UUID: "u1"}, {UUID: ""}}, // empty UUID → no directory
		Radios:    []model.InternetRadioStation{{StreamURL: "http://s"}},
	}

	live := liveAssetKeys(ids)

	want := map[string]string{
		assetstore.KindAlbum:    assetkey.Album("al", "ar", "rel"),
		assetstore.KindGenre:    assetkey.Genre("Rock"),
		assetstore.KindPlaylist: assetkey.Playlist("u1"),
		assetstore.KindRadio:    assetkey.Radio("http://s"),
	}
	for kind, key := range want {
		if _, ok := live[kind][key]; !ok {
			t.Errorf("live[%s] missing key %s", kind, key)
		}
	}

	// An MBID artist occupies BOTH its MBID slot and its name-hash slot, since
	// covers can have been stored under either (the delete path clears both).
	artistKeys := live[assetstore.KindArtist]
	if _, ok := artistKeys[assetkey.Artist("mbid-q", "queen")]; !ok {
		t.Error("artist MBID slot missing")
	}
	if _, ok := artistKeys[assetkey.Artist("", "queen")]; !ok {
		t.Error("MBID artist's name-hash slot missing")
	}
	if _, ok := artistKeys[assetkey.Artist("", "noid")]; !ok {
		t.Error("no-MBID artist name-hash slot missing")
	}

	// The empty-UUID playlist can own no directory, so it contributes no key.
	if _, ok := live[assetstore.KindPlaylist][""]; ok {
		t.Error("empty playlist key must not be in the live set")
	}
}

// prune must be triggerable and schedulable, so it belongs in the user-facing
// task catalogue (unlike reindex, which is enqueued by the editor).
func TestPruneIsAnAvailableTask(t *testing.T) {
	if !TaskNameExists(PruneTaskName) {
		t.Fatal("prune must be in AvailableTasks so it can be triggered and scheduled")
	}
}

func TestNewPruneTaskFnRemovesOrphansKeepsLiveEditorAndManual(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()

	album := model.Album{Name: "Al", NameNorm: "al", AlbumArtistNorm: "ar", MBReleaseID: "rel"}
	db.Create(&album)
	artist := model.Artist{Name: "Queen", NameNorm: "queen", MBArtistID: "mbid-q"}
	db.Create(&artist)
	db.Create(&model.Genre{Name: "Rock"})
	db.Create(&model.Playlist{UUID: "u1", Name: "P"})
	db.Create(&model.InternetRadioStation{Name: "R", StreamURL: "http://s"})

	imgRoot := t.TempDir()
	assetRoot := t.TempDir()
	images := imagecache.New(imgRoot)
	assets := assetstore.New(assetRoot)

	liveAlbumKey := assetkey.AlbumOf(&album)
	liveArtistKey := assetkey.ArtistOf(&artist)

	// Live derivatives + a live stored cover — all must survive.
	seedCacheEntry(t, imgRoot, assetstore.KindAlbum, liveAlbumKey)
	seedCacheEntry(t, imgRoot, assetstore.KindArtist, liveArtistKey)
	if err := assets.PutAuto(assetstore.KindArtist, liveArtistKey, "jpg", []byte("live")); err != nil {
		t.Fatal(err)
	}

	// Orphan derivatives + an orphan auto stored cover — all must go.
	seedCacheEntry(t, imgRoot, assetstore.KindAlbum, "deadalbum")
	seedCacheEntry(t, imgRoot, assetstore.KindGenre, "deadgenre")
	if err := assets.PutAuto(assetstore.KindAlbum, "deadalbum", "jpg", []byte("orphan-auto")); err != nil {
		t.Fatal(err)
	}

	// An orphan whose stored cover was hand-uploaded — must be preserved.
	if err := assets.PutManual(assetstore.KindAlbum, "deadmanual", "jpg", []byte("upload")); err != nil {
		t.Fatal(err)
	}

	// The metadata editor's thumbnails live under a non-entity cache kind — must survive.
	seedCacheEntry(t, imgRoot, "editor", "some-source-file")

	fn := NewPruneTaskFn(s, images, assets)
	if err := fn(context.Background(), slog.Default(), &progressSpy{}); err != nil {
		t.Fatalf("prune fn: %v", err)
	}

	// Live survive.
	if !cacheEntryExists(imgRoot, assetstore.KindAlbum, liveAlbumKey) {
		t.Error("live album derivative was removed")
	}
	if !cacheEntryExists(imgRoot, assetstore.KindArtist, liveArtistKey) {
		t.Error("live artist derivative was removed")
	}
	if _, ok := assets.Get(assetstore.KindArtist, liveArtistKey); !ok {
		t.Error("live stored cover was removed")
	}

	// Orphans gone.
	if cacheEntryExists(imgRoot, assetstore.KindAlbum, "deadalbum") {
		t.Error("orphan album derivative survived")
	}
	if cacheEntryExists(imgRoot, assetstore.KindGenre, "deadgenre") {
		t.Error("orphan genre derivative survived")
	}
	if _, ok := assets.Get(assetstore.KindAlbum, "deadalbum"); ok {
		t.Error("orphan auto stored cover survived")
	}

	// Manual upload of an orphan preserved.
	if _, ok := assets.Get(assetstore.KindAlbum, "deadmanual"); !ok {
		t.Error("orphan manual upload must be preserved")
	}

	// Editor thumbnails untouched.
	if !cacheEntryExists(imgRoot, "editor", "some-source-file") {
		t.Error("editor thumbnail must survive the prune")
	}
}

// TestNewPruneTaskFnHonoursContextCancellation proves the task stops early when
// its context is cancelled instead of running the whole sweep.
func TestNewPruneTaskFnHonoursContextCancellation(t *testing.T) {
	s := newTestStore(t)
	images := imagecache.New(t.TempDir())
	assets := assetstore.New(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewPruneTaskFn(s, images, assets)(ctx, slog.Default(), &progressSpy{})
	if err == nil {
		t.Fatal("expected a context error when the task is cancelled")
	}
}

// progressSpy records the progress calls the task makes, so a test can assert it
// reports a total, a stage per kind, and one increment per kind.
type progressSpy struct {
	total  int64
	stages []string
	incs   int
}

func (p *progressSpy) SetTotal(t int64)  { p.total = t }
func (p *progressSpy) Set(int64)         {}
func (p *progressSpy) Inc(int64) int64   { p.incs++; return 0 }
func (p *progressSpy) SetStage(m string) { p.stages = append(p.stages, m) }

func TestNewPruneTaskFnReportsProgress(t *testing.T) {
	s := newTestStore(t)
	images := imagecache.New(t.TempDir())
	assets := assetstore.New(t.TempDir())

	spy := &progressSpy{}
	if err := NewPruneTaskFn(s, images, assets)(context.Background(), slog.Default(), spy); err != nil {
		t.Fatalf("prune fn: %v", err)
	}

	if spy.total != int64(len(pruneKinds)) {
		t.Errorf("SetTotal = %d, want %d", spy.total, len(pruneKinds))
	}
	if spy.incs != len(pruneKinds) {
		t.Errorf("Inc called %d times, want %d", spy.incs, len(pruneKinds))
	}
	if len(spy.stages) != len(pruneKinds) {
		t.Errorf("SetStage called %d times, want %d", len(spy.stages), len(pruneKinds))
	}
}

func seedCacheEntry(t *testing.T, root, kind, key string) {
	t.Helper()
	dir := filepath.Join(root, kind, key)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.abc123.200.webp"), []byte("x"), 0o640); err != nil {
		t.Fatalf("write derivative: %v", err)
	}
}

func cacheEntryExists(root, kind, key string) bool {
	_, err := os.Stat(filepath.Join(root, kind, key))
	return err == nil
}
