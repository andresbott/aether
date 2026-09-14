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

func TestLiveAssetKeysSplitsArtistSlotsBetweenStores(t *testing.T) {
	ids := store.AssetIdentities{
		Albums:    []model.Album{{NameNorm: "al", AlbumArtistNorm: "ar", MBReleaseID: "rel"}},
		Artists:   []model.Artist{{NameNorm: "queen", MBArtistID: "mbid-q"}, {NameNorm: "noid"}},
		Genres:    []model.Genre{{Name: "Rock"}},
		Playlists: []model.Playlist{{UUID: "u1"}, {UUID: ""}}, // empty UUID → no directory
		Radios:    []model.InternetRadioStation{{StreamURL: "http://s"}},
	}

	images, assets := liveAssetKeys(ids)

	// Non-artist kinds are identical in both stores.
	for kind, key := range map[string]string{
		assetstore.KindAlbum:    assetkey.Album("al", "ar", "rel"),
		assetstore.KindGenre:    assetkey.Genre("Rock"),
		assetstore.KindPlaylist: assetkey.Playlist("u1"),
		assetstore.KindRadio:    assetkey.Radio("http://s"),
	} {
		if _, ok := images[kind][key]; !ok {
			t.Errorf("images live[%s] missing key %s", kind, key)
		}
		if _, ok := assets[kind][key]; !ok {
			t.Errorf("assets live[%s] missing key %s", kind, key)
		}
	}

	mbid := assetkey.Artist("mbid-q", "queen") // ArtistOf for the MBID artist
	queenNameHash := assetkey.Artist("", "queen")
	noidNameHash := assetkey.Artist("", "noid")

	// Asset store keeps BOTH slots — the name-hash slot is a served fallback.
	for _, k := range []string{mbid, queenNameHash, noidNameHash} {
		if _, ok := assets[assetstore.KindArtist][k]; !ok {
			t.Errorf("assets artist live missing %s", k)
		}
	}

	// Image cache keeps only the current ArtistOf slot: the MBID artist's dead
	// name-hash slot is NOT live, so prune reclaims its drift derivatives.
	if _, ok := images[assetstore.KindArtist][mbid]; !ok {
		t.Error("images artist live missing the MBID slot")
	}
	if _, ok := images[assetstore.KindArtist][queenNameHash]; ok {
		t.Error("images artist live must NOT include the MBID artist's name-hash slot")
	}
	// A no-MBID artist's ArtistOf IS its name-hash, so that slot stays live in images.
	if _, ok := images[assetstore.KindArtist][noidNameHash]; !ok {
		t.Error("images artist live missing the no-MBID artist's slot")
	}

	// The empty-UUID playlist can own no directory, so it contributes no key.
	if _, ok := assets[assetstore.KindPlaylist][""]; ok {
		t.Error("empty playlist key must not be in the asset live set")
	}
	if _, ok := images[assetstore.KindPlaylist][""]; ok {
		t.Error("empty playlist key must not be in the image live set")
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

// An artist that gains an MBID leaves its old name-hash-slot derivatives
// orphaned: new derivatives cache under ArtistOf (the MBID), so the name-hash
// IMAGE-cache slot is dead and prune must reclaim it. The name-hash STORED cover
// must survive, though — artistCoverMeta still serves it as a fallback, so it is
// a live source, not an orphan.
func TestNewPruneTaskFnReclaimsMBIDDriftImageDerivatives(t *testing.T) {
	s := newTestStore(t)
	db := s.DB()
	artist := model.Artist{Name: "Queen", NameNorm: "queen", MBArtistID: "mbid-q"}
	db.Create(&artist)

	imgRoot := t.TempDir()
	assetRoot := t.TempDir()
	images := imagecache.New(imgRoot)
	assets := assetstore.New(assetRoot)

	mbidKey := assetkey.ArtistOf(&artist)               // current derivative key
	nameHashKey := assetkey.Artist("", artist.NameNorm) // dead drift slot
	if mbidKey == nameHashKey {
		t.Fatal("test needs the MBID and name-hash keys to differ")
	}

	seedCacheEntry(t, imgRoot, assetstore.KindArtist, mbidKey)     // live
	seedCacheEntry(t, imgRoot, assetstore.KindArtist, nameHashKey) // dead drift
	// The name-hash STORED cover is still a live fallback source.
	if err := assets.PutAuto(assetstore.KindArtist, nameHashKey, "jpg", []byte("fallback")); err != nil {
		t.Fatal(err)
	}

	if err := NewPruneTaskFn(s, images, assets)(context.Background(), slog.Default(), &progressSpy{}); err != nil {
		t.Fatalf("prune fn: %v", err)
	}

	if !cacheEntryExists(imgRoot, assetstore.KindArtist, mbidKey) {
		t.Error("current MBID-slot derivatives must be kept")
	}
	if cacheEntryExists(imgRoot, assetstore.KindArtist, nameHashKey) {
		t.Error("dead name-hash-slot derivatives must be reclaimed after an MBID gain")
	}
	if _, ok := assets.Get(assetstore.KindArtist, nameHashKey); !ok {
		t.Error("the name-hash STORED cover is a live fallback and must be kept")
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
