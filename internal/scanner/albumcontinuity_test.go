// internal/scanner/albumcontinuity_test.go
package scanner_test

import (
	"context"
	"path/filepath"
	"testing"

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
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

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
	if after.CoverPath != before.CoverPath {
		t.Fatalf("cover_path changed on retag (was %q, now %q): cover resolution broke", before.CoverPath, after.CoverPath)
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

// A batch that disagrees with itself is a split, not a rename: the row keeps
// its identity and the moved track gets a new row.
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

	// One of the three files now claims a different album; the other two do not.
	reader.perPath = map[string]string{
		filepath.Join(dir, "Apocalyptica/Cult/03.mp3"): "Cult (Bonus)",
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var albums []model.Album
	if err := st.DB().Order("id").Find(&albums).Error; err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 rows after the split, got %d: %+v", len(albums), albums)
	}
	if albums[0].ID != before.ID || albums[0].NameNorm != "cult" {
		t.Fatalf("the majority must keep the original row untouched: %+v", albums[0])
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
// star are as high as they can be.
func TestReconcileKeepsTheLargerAlbumWhenTwoAlbumsMerge(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/CD1/01.mp3",
		"Apocalyptica/CD1/02.mp3",
		"Apocalyptica/CD1/03.mp3",
		"Apocalyptica/CD2/04.mp3",
	})
	seedLibrary(t, st, dir, nil)

	disc2 := filepath.Join(dir, "Apocalyptica/CD2/04.mp3")
	reader := &retagReader{
		album:       "Cult (Disc 1)",
		albumArtist: "Apocalyptica",
		perPath:     map[string]string{disc2: "Cult (Disc 2)"},
	}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var big model.Album
	if err := st.DB().Where("name_norm = ?", "cult (disc 1)").First(&big).Error; err != nil {
		t.Fatal(err)
	}

	// Both discs are retagged to one album name.
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
	if after.CoverPath != want {
		t.Fatalf("expected the multi-disc cover to survive, wanted %q got %q", want, after.CoverPath)
	}
}
