// internal/scanner/artistimage_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// mbidGainReader is a tag reader whose artist MBID can be changed between scans,
// standing in for the metadata editor or MusicBrainz Picard writing new tags.
type mbidGainReader struct {
	mbid string
}

func (r *mbidGainReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r *mbidGainReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	var mbids []string
	if r.mbid != "" {
		mbids = []string{r.mbid}
	}
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{"Test Artist"},
		AlbumArtist: []string{"Test Artist"},
		Album:       filepath.Base(filepath.Dir(absPath)),
		MBArtistID:  mbids,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}

// The scanner records the artist-folder image on the artist row so cover
// resolution does not have to touch the filesystem per request.
func TestScannerRecordsArtistFolderImage(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Test Artist/Album One/01.mp3",
		"Test Artist/Album One/cover.jpg",
		"Test Artist/artist.jpg",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artist model.Artist
	if err := st.DB().Where("name_norm = ?", "test artist").First(&artist).Error; err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Test Artist/artist.jpg")
	if artist.ImagePath != want {
		t.Fatalf("ImagePath = %q, want %q", artist.ImagePath, want)
	}
}

// A library that is not laid out as <collection>/<artist>/<album> must leave
// ImagePath empty rather than adopt the album's own folder image.
func TestScannerLeavesArtistImageEmptyWithoutArtistFolder(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Album One/01.mp3",
		"Album One/folder.jpg",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artist model.Artist
	if err := st.DB().Where("name_norm = ?", "test artist").First(&artist).Error; err != nil {
		t.Fatal(err)
	}
	if artist.ImagePath != "" {
		t.Fatalf("ImagePath = %q, want empty", artist.ImagePath)
	}
}

// A recorded path that has since gone away must be dropped, not kept forever.
func TestScannerClearsStaleArtistImagePath(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Test Artist/Album One/01.mp3",
		"Test Artist/artist.jpg",
	})
	seedLibrary(t, st, dir, nil)

	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Test Artist/artist.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artist model.Artist
	if err := st.DB().Where("name_norm = ?", "test artist").First(&artist).Error; err != nil {
		t.Fatal(err)
	}
	if artist.ImagePath != "" {
		t.Fatalf("ImagePath = %q, want empty after the file was removed", artist.ImagePath)
	}
}

// Asset re-key hook test: a manual artist cover stored while the artist was
// unmatched must still resolve after the artist gains an MBID. The manual upload
// must also continue to outrank an auto-fetched image.
func TestReconcileRekeysArtistImagesWhenTheArtistGainsAnMBID(t *testing.T) {
	st := testScanStore(t)
	assetRoot := t.TempDir()
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Nirvana/Nevermind/01.mp3",
	})
	seedLibrary(t, st, dir, nil)

	// Initial scan: artist has no MBID.
	reader := &mbidGainReader{mbid: ""}
	assets := assetstore.New(assetRoot)
	cfg := scanner.Config{AssetRekeyer: assets}
	s := scanner.New(cfg, st, reader)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artist model.Artist
	if err := st.DB().Where("name_norm = ?", "test artist").First(&artist).Error; err != nil {
		t.Fatal(err)
	}
	if artist.MBArtistID != "" {
		t.Fatalf("fixture: artist must start with no MBID, got %q", artist.MBArtistID)
	}

	// Store a manual cover under the name-hash key.
	oldKey := assetkey.Artist("", artist.NameNorm)
	if err := assets.PutManual(assetstore.KindArtist, oldKey, "jpg", []byte("manual cover")); err != nil {
		t.Fatal(err)
	}
	if _, ok := assets.Get(assetstore.KindArtist, oldKey); !ok {
		t.Fatal("fixture: manual cover must be present under old key")
	}

	// Rescan with tags that now carry an MBID.
	reader.mbid = "mbid-nirvana"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// The artist row must have gained the MBID.
	if err := st.DB().First(&artist, artist.ID).Error; err != nil {
		t.Fatal(err)
	}
	if artist.MBArtistID != "mbid-nirvana" {
		t.Fatalf("expected artist to gain MBID, got %q", artist.MBArtistID)
	}

	// The cover must now resolve under the MBID key and NOT the old name-hash key.
	newKey := assetkey.ArtistOf(&artist)
	if path, ok := assets.Get(assetstore.KindArtist, newKey); !ok {
		t.Fatalf("cover not found under new key %q", newKey)
	} else {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "manual cover" {
			t.Fatalf("cover under new key has wrong content")
		}
	}
	if _, ok := assets.Get(assetstore.KindArtist, oldKey); ok {
		t.Fatalf("cover still resolves under old key %q; the re-key did not move it", oldKey)
	}

	// A manual upload must still outrank an auto-fetched image.
	if err := assets.PutAuto(assetstore.KindArtist, newKey, "jpg", []byte("auto cover")); err != nil {
		t.Fatal(err)
	}
	if path, manual, ok := assets.GetEntry(assetstore.KindArtist, newKey); !ok {
		t.Fatal("cover not found after adding auto variant")
	} else if !manual {
		t.Fatal("manual cover must outrank auto-fetched")
	} else {
		data, _ := os.ReadFile(path)
		if string(data) != "manual cover" {
			t.Fatal("wrong cover resolved after adding auto variant")
		}
	}
}

// recordingRekeyer wraps an AssetRekeyer and records each Rekey call, for
// testing that the hook was invoked with expected keys.
type recordingRekeyer struct {
	delegate scanner.AssetRekeyer
	calls    []rekeyCall
}

type rekeyCall struct {
	kind   string
	oldKey string
	newKey string
	err    error
}

func (r *recordingRekeyer) Rekey(kind, oldKey, newKey string) error {
	err := r.delegate.Rekey(kind, oldKey, newKey)
	r.calls = append(r.calls, rekeyCall{kind: kind, oldKey: oldKey, newKey: newKey, err: err})
	return err
}

// A malformed MBID falls back to hashing, so old and new keys may be equal.
// The re-key must handle that gracefully and log nothing spurious.
func TestReconcileToleratesMalformedMBIDWhereKeysCoincide(t *testing.T) {
	st := testScanStore(t)
	assetRoot := t.TempDir()
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Muse/Absolution/01.mp3",
	})
	seedLibrary(t, st, dir, nil)

	// Initial scan: artist has no MBID.
	reader := &mbidGainReader{mbid: ""}
	assets := assetstore.New(assetRoot)
	cfg := scanner.Config{AssetRekeyer: assets}
	s := scanner.New(cfg, st, reader)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artist model.Artist
	if err := st.DB().Where("name_norm = ?", "test artist").First(&artist).Error; err != nil {
		t.Fatal(err)
	}

	// Store a manual cover under the name-hash key.
	key := assetkey.Artist("", artist.NameNorm)
	if err := assets.PutManual(assetstore.KindArtist, key, "jpg", []byte("manual cover")); err != nil {
		t.Fatal(err)
	}

	// Rescan with a malformed MBID (contains "/" which is not key-safe).
	// assetkey.Artist will fall back to hashing, so old and new keys are equal.
	// Wrap assets in a recording decorator to verify the hook ran.
	recorder := &recordingRekeyer{delegate: assets}
	s = scanner.New(scanner.Config{AssetRekeyer: recorder}, st, reader)
	reader.mbid = "malformed/mbid"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// The scan must succeed and the row must carry the malformed MBID.
	if err := st.DB().First(&artist, artist.ID).Error; err != nil {
		t.Fatal(err)
	}
	if artist.MBArtistID != "malformed/mbid" {
		t.Fatalf("expected artist to carry malformed MBID, got %q", artist.MBArtistID)
	}

	// The cover must still be intact (no spurious move attempted).
	if path, ok := assets.Get(assetstore.KindArtist, key); !ok {
		t.Fatal("cover not found after malformed MBID")
	} else {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "manual cover" {
			t.Fatal("cover has wrong content after malformed MBID")
		}
	}

	// The hook must have been called exactly once with oldKey == newKey.
	if len(recorder.calls) != 1 {
		t.Fatalf("expected 1 Rekey call, got %d", len(recorder.calls))
	}
	call := recorder.calls[0]
	if call.kind != assetstore.KindArtist {
		t.Fatalf("expected kind %q, got %q", assetstore.KindArtist, call.kind)
	}
	if call.oldKey != call.newKey {
		t.Fatalf("expected oldKey == newKey (the no-op path), got old=%q new=%q", call.oldKey, call.newKey)
	}
	if call.oldKey != key {
		t.Fatalf("expected key %q, got old=%q new=%q", key, call.oldKey, call.newKey)
	}
	if call.err != nil {
		t.Fatalf("expected Rekey to return nil, got %v", call.err)
	}
}
