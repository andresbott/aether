// internal/scanner/albumcontinuity_test.go
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
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

// retagReader is a tag reader whose album identity can be changed between
// scans, standing in for the metadata editor writing new tags to the files.
// perPath overrides the album name of individual files, for the cases where a
// batch disagrees with itself.
type retagReader struct {
	album       string
	albumArtist string
	mbReleaseID string
	perPath     map[string]string
}

func (r *retagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r *retagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	album := r.album
	if v, ok := r.perPath[absPath]; ok {
		album = v
	}
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{r.albumArtist},
		AlbumArtist: []string{r.albumArtist},
		Album:       album,
		MBReleaseID: r.mbReleaseID,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}

// theOnlyAlbum fails unless the DB holds exactly one album row, and returns it.
func theOnlyAlbum(t *testing.T, st *store.Store) model.Album {
	t.Helper()
	var albums []model.Album
	if err := st.DB().Find(&albums).Error; err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 {
		t.Fatalf("expected exactly 1 album row, got %d: %+v", len(albums), albums)
	}
	return albums[0]
}

// The core promise: retagging every track of an album keeps the row, so
// everything keyed on albums.id survives — the manual cover in the asset store
// (handlers/subsonic/albums.go keys on it), stars, created_at and therefore the
// "newest" list and the discovery feed, plus every client-cached /album/:id.
func TestReconcileKeepsTheAlbumIDWhenTheWholeAlbumIsRetagged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
		"Apocalyptica/Cult/03.mp3",
		"Apocalyptica/Cult/cover.jpg",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	if before.CoverPath == "" {
		t.Fatal("fixture must produce a cover for the later cover_path assertion to mean anything")
	}

	// The editor fixes the misspelled album artist on every file of the album.
	reader.albumArtist = "Apocalyptica"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("album id changed on retag: was %d, now %d", before.ID, after.ID)
	}
	if after.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", after.AlbumArtistNorm)
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at changed on retag (%v -> %v): the album would resurface in \"newest\" and the discovery feed",
			before.CreatedAt, after.CreatedAt)
	}
	// Guards that planAlbumContinuity writes identity columns only. If the
	// pre-pass blanked or rewrote cover_path this would fail. Does not prove
	// continuity preserves the cover — reconcile re-detects it from disk anyway.
	if after.CoverPath != before.CoverPath {
		t.Fatalf("cover_path changed on retag (was %q, now %q): the pre-pass wrote a column it should not have", before.CoverPath, after.CoverPath)
	}

	var trackCount int64
	if err := st.DB().Model(&model.Track{}).Where("album_id = ?", after.ID).Count(&trackCount).Error; err != nil {
		t.Fatal(err)
	}
	if trackCount != 3 {
		t.Fatalf("expected all 3 tracks on the retagged row, got %d", trackCount)
	}
}

// The identify-album flow: the whole selection gains an MBReleaseID at once.
// This is the most common churn trigger in practice.
func TestReconcileKeepsTheAlbumIDWhenTheAlbumGainsAnMBID(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	reader.mbReleaseID = "e1b2c3d4-0000-0000-0000-000000000001"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("album id changed when the album gained an MBID: was %d, now %d", before.ID, after.ID)
	}
	if after.MBReleaseID != reader.mbReleaseID {
		t.Fatalf("expected the row to carry the new MBID, got %q", after.MBReleaseID)
	}
}

// Only part of an album was edited: the album genuinely splits, and the
// original row must keep its id and its remaining tracks. This is the metadata
// editor's real shape — it rescans exactly the paths it wrote.
func TestRescanPathsSplitsAnAlbumWhenOnlySomeTracksAreRetagged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
		"Apocalyptica/Cult/03.mp3",
	})
	lib := seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	// The editor moves ONE file to a different album and rescans just that file.
	edited := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader.album = "Cult (Single)"
	if _, err := s.RescanPaths(context.Background(), lib.ID, []string{edited}); err != nil {
		t.Fatal(err)
	}

	var albums []model.Album
	if err := st.DB().Order("id").Find(&albums).Error; err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected the album to split into 2 rows, got %d: %+v", len(albums), albums)
	}
	if albums[0].ID != before.ID || albums[0].NameNorm != "cult" {
		t.Fatalf("the untouched album must keep its row: %+v", albums[0])
	}

	var remaining int64
	if err := st.DB().Model(&model.Track{}).Where("album_id = ?", before.ID).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("expected 2 tracks left on the original album, got %d", remaining)
	}
}

// A batch that disagrees with itself is a split, not a rename: every track moves
// but they disagree on the target, so no row is retagged and the originals die.
func TestReconcileDoesNotRetagWhenTheBatchDisagrees(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
		"Apocalyptica/Cult/03.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	// Every track moves to a different new identity: two targets, both different
	// from what the row currently holds. The planner declines (batch disagrees),
	// falls back to FindOrCreate, and both targets get fresh rows.
	reader.perPath = map[string]string{
		filepath.Join(dir, "Apocalyptica/Cult/01.mp3"): "One",
		filepath.Join(dir, "Apocalyptica/Cult/02.mp3"): "Two",
		filepath.Join(dir, "Apocalyptica/Cult/03.mp3"): "Two",
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var albums []model.Album
	if err := st.DB().Order("id").Find(&albums).Error; err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 rows (One + Two), got %d: %+v", len(albums), albums)
	}
	// The decisive assertion: neither carries the original album's id. If either
	// did, it would mean the planner renamed on a split, which is the bug.
	for _, album := range albums {
		if album.ID == before.ID {
			t.Fatalf("original album %d survived; the planner renamed on a disagreeing batch: %+v", before.ID, album)
		}
	}
}

// Retagging an album onto an identity another row already holds is a merge:
// the tracks move to the existing row. Guard that continuity planning does not
// try to rename into a taken identity.
func TestReconcileMergesIntoAnExistingAlbumIdentity(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Other/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	move := filepath.Join(dir, "Apocalyptica/Other/02.mp3")
	reader := &retagReader{
		album:       "Cult",
		albumArtist: "Apocalyptica",
		perPath:     map[string]string{move: "Reflections"},
	}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var cult model.Album
	if err := st.DB().Where("name_norm = ?", "cult").First(&cult).Error; err != nil {
		t.Fatal(err)
	}

	// "Reflections" is retagged to "Cult", which already exists.
	reader.perPath = map[string]string{move: "Cult"}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != cult.ID {
		t.Fatalf("expected the tracks to merge into the existing album %d, got %d", cult.ID, after.ID)
	}

	var merged int64
	if err := st.DB().Model(&model.Track{}).Where("album_id = ?", after.ID).Count(&merged).Error; err != nil {
		t.Fatal(err)
	}
	if merged != 2 {
		t.Fatalf("expected both tracks on the surviving row, got %d", merged)
	}
}

// Two albums collapsing into one identity in a single batch: one row must die,
// but which one must not depend on the order the tag readers finished in. The
// album with more tracks keeps its id, so the odds of preserving a cover or a
// star are as high as they can be. Three claimants with distinct track counts
// makes an arbitrary pick wrong ~2/3 of the time.
func TestReconcileKeepsTheLargerAlbumWhenTwoAlbumsMerge(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/CD1/01.mp3",
		"Apocalyptica/CD1/02.mp3",
		"Apocalyptica/CD1/03.mp3",
		"Apocalyptica/CD2/04.mp3",
		"Apocalyptica/CD2/05.mp3",
		"Apocalyptica/CD3/06.mp3",
	})
	seedLibrary(t, st, dir, nil)

	cd2a := filepath.Join(dir, "Apocalyptica/CD2/04.mp3")
	cd2b := filepath.Join(dir, "Apocalyptica/CD2/05.mp3")
	cd3 := filepath.Join(dir, "Apocalyptica/CD3/06.mp3")
	reader := &retagReader{
		album:       "Cult (Disc 1)",
		albumArtist: "Apocalyptica",
		perPath: map[string]string{
			cd2a: "Cult (Disc 2)",
			cd2b: "Cult (Disc 2)",
			cd3:  "Cult (Disc 3)",
		},
	}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var big model.Album
	if err := st.DB().Where("name_norm = ?", "cult (disc 1)").First(&big).Error; err != nil {
		t.Fatal(err)
	}

	// All three discs are retagged to one album name.
	reader.album = "Cult"
	reader.perPath = nil
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != big.ID {
		t.Fatalf("expected the 3-track album %d to keep its id, got %d", big.ID, after.ID)
	}
	if after.NameNorm != "cult" {
		t.Fatalf("expected the surviving row to carry the new identity, got %q", after.NameNorm)
	}
}

// The editor's real path: write tags, then RescanPaths the files just written.
// A star and created_at must both come out the other side, because
// DeleteOrphanedAggregates deletes the star row of an album that dies
// (internal/store/scan_helpers.go) and created_at drives both the "newest"
// album list and the discovery feed's recency term.
func TestRescanPathsRetagPreservesStarsAndCreatedAt(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	lib := seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	if err := st.Star("alice", "album", before.ID); err != nil {
		t.Fatal(err)
	}

	// The editor renames the album on both files and rescans them.
	reader.album = "Cult - Special Edition"
	paths := []string{
		filepath.Join(dir, "Apocalyptica/Cult/01.mp3"),
		filepath.Join(dir, "Apocalyptica/Cult/02.mp3"),
	}
	if _, err := s.RescanPaths(context.Background(), lib.ID, paths); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("album id changed on a RescanPaths retag: was %d, now %d", before.ID, after.ID)
	}
	if after.NameNorm != "cult - special edition" {
		t.Fatalf("expected the new name on the row, got %q", after.NameNorm)
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at changed on retag (%v -> %v): the album would resurface in \"newest\" and the discovery feed",
			before.CreatedAt, after.CreatedAt)
	}

	var stars int64
	if err := st.DB().Table("starred_items").
		Where("item_type = ? AND item_id = ?", "album", before.ID).
		Count(&stars).Error; err != nil {
		t.Fatal(err)
	}
	if stars != 1 {
		t.Fatalf("expected the star to survive the retag, found %d rows", stars)
	}
}

// Asset re-key hook tests: the manual album cover must follow the row when
// planAlbumContinuity retags it in place.

func TestReconcileRekeysAlbumImagesWhenTheAlbumIsRetagged(t *testing.T) {
	st := testScanStore(t)
	assetRoot := t.TempDir()
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	assets := assetstore.New(assetRoot)
	cfg := scanner.Config{AssetRekeyer: assets}
	s := scanner.New(cfg, st, reader)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	album := theOnlyAlbum(t, st)

	// Store a manual cover under the old identity's key.
	oldKey := assetkey.Album("cult", "apocaliptica", "")
	if err := assets.PutManual("album", oldKey, "jpg", []byte("old cover")); err != nil {
		t.Fatal(err)
	}
	if _, ok := assets.Get("album", oldKey); !ok {
		t.Fatal("fixture: manual cover must be present under old key")
	}

	// The editor fixes the misspelled album artist on every file.
	reader.albumArtist = "Apocalyptica"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// The album row must have been retagged (continuity preserved the id).
	after := theOnlyAlbum(t, st)
	if after.ID != album.ID {
		t.Fatalf("expected album id to survive retag, was %d now %d", album.ID, after.ID)
	}
	if after.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", after.AlbumArtistNorm)
	}

	// The cover must now resolve under the new identity's key and NOT the old one.
	newKey := assetkey.Album("cult", "apocalyptica", "")
	if path, ok := assets.Get("album", newKey); !ok {
		t.Fatalf("cover not found under new key %q", newKey)
	} else {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "old cover" {
			t.Fatalf("cover under new key has wrong content")
		}
	}
	if _, ok := assets.Get("album", oldKey); ok {
		t.Fatalf("cover still resolves under old key %q; the re-key did not move it", oldKey)
	}
}

func TestReconcileToleratesAnOccupiedDestinationKey(t *testing.T) {
	st := testScanStore(t)
	assetRoot := t.TempDir()
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	assets := assetstore.New(assetRoot)
	cfg := scanner.Config{AssetRekeyer: assets}
	s := scanner.New(cfg, st, reader)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	oldKey := assetkey.Album("cult", "apocaliptica", "")
	newKey := assetkey.Album("cult", "apocalyptica", "")
	if err := assets.PutManual("album", oldKey, "jpg", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := assets.PutManual("album", newKey, "jpg", []byte("new")); err != nil {
		t.Fatal(err)
	}

	// Retag: the destination key already holds images (a merge case).
	reader.albumArtist = "Apocalyptica"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// The album row must have been retagged (the scan succeeded).
	album := theOnlyAlbum(t, st)
	if album.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", album.AlbumArtistNorm)
	}

	// Both images must still be intact (the move was refused, not forced).
	if path, ok := assets.Get("album", oldKey); !ok {
		t.Fatal("old key's image was destroyed")
	} else if data, _ := os.ReadFile(path); string(data) != "old" {
		t.Fatal("old key's image has wrong content")
	}
	if path, ok := assets.Get("album", newKey); !ok {
		t.Fatal("new key's image was destroyed")
	} else if data, _ := os.ReadFile(path); string(data) != "new" {
		t.Fatal("new key's image has wrong content")
	}
}

func TestReconcileRetagsAlbumWithNoAssetRekeyer(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	// No AssetRekeyer in Config: the hook must be optional.
	s := scanner.New(scanner.Config{}, st, reader)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	reader.albumArtist = "Apocalyptica"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// The album row must have been retagged: a nil rekeyer does not break continuity.
	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("expected album id to survive retag, was %d now %d", before.ID, after.ID)
	}
	if after.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", after.AlbumArtistNorm)
	}
}

// A multi-disc release is one album row spanning two directories
// (docs/agents/scanning.md). Retagging it wholesale must keep the row, and the
// existing multi-disc cover behaviour must not move.
func TestReconcileKeepsTheAlbumIDWhenAMultiDiscAlbumIsRetagged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/CD 1/01.mp3",
		"Apocalyptica/Cult/CD 1/cover.jpg",
		"Apocalyptica/Cult/CD 2/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	reader.album = "Cult (Remastered)"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("multi-disc album id changed on retag: was %d, now %d", before.ID, after.ID)
	}
	want := filepath.Join(dir, "Apocalyptica/Cult/CD 1/cover.jpg")
	// Guards that planAlbumContinuity writes identity columns only. Does not
	// prove continuity preserves the cover — reconcile would re-detect the same
	// cover.jpg from disk even if the row had been recreated.
	if after.CoverPath != want {
		t.Fatalf("cover_path changed (wanted %q got %q): the pre-pass wrote a column it should not have", want, after.CoverPath)
	}
}
