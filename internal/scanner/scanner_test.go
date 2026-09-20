// internal/scanner/scanner_test.go
package scanner_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testScanStore(t *testing.T) *store.Store {
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

// seedFolder declares dir as a scan folder for a test. The name mirrors what the
// old library fixture used, so expectations on it read the same.
func seedFolder(path string, excludes []string) scanfolder.Folder {
	return scanfolder.Folder{
		Name:            filepath.Base(path) + "-lib",
		Path:            path,
		ExcludePatterns: excludes,
		FollowSymlinks:  true,
	}
}

// newScanner builds a scanner over the given scan folders. The set is immutable,
// exactly like the config it comes from: a test that adds, removes or moves a
// folder between two scans builds a NEW scanner, which is what a restart with an
// edited config does.
func newScanner(t *testing.T, st *store.Store, reader tags.Reader, folders ...scanfolder.Folder) *scanner.Scanner {
	t.Helper()
	set, err := scanfolder.NewSet(folders)
	if err != nil {
		t.Fatal(err)
	}
	return scanner.New(scanner.Config{Folders: set}, st, reader)
}

type fakeTagReader struct{}

func (fakeTagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }
func (fakeTagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	name := filepath.Base(absPath)
	dir := filepath.Base(filepath.Dir(absPath))
	return tags.Metadata{
		Title:       name,
		Artist:      []string{"Test Artist"},
		AlbumArtist: []string{"Test Artist"},
		Album:       dir,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}

// TestScanWithNilFolderSet is the only coverage of a NIL Config.Folders:
// newScanner (this file's helper) always builds a non-nil set via
// scanfolder.NewSet, so this must keep constructing the Scanner directly with
// scanner.New(scanner.Config{}, …) rather than switching to that helper.
func TestScanWithNilFolderSet(t *testing.T) {
	st := testScanStore(t)
	s := scanner.New(scanner.Config{}, st, fakeTagReader{})
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected 0 tracks processed, got %d", stats.TracksProcessed)
	}
}

func TestScannerFullScan(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Test Artist/Album One/01-track.mp3",
		"Test Artist/Album One/02-track.mp3",
		"Test Artist/Album Two/01-track.flac",
	})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 3 {
		t.Fatalf("expected 3 tracks processed, got %d", stats.TracksProcessed)
	}

	var trackCount int64
	st.DB().Model(&model.Track{}).Count(&trackCount)
	if trackCount != 3 {
		t.Fatalf("expected 3 tracks in DB, got %d", trackCount)
	}

	// Every track should carry the scan folder's name.
	var withFolder int64
	st.DB().Model(&model.Track{}).Where("scan_folder = ?", folder.Name).Count(&withFolder)
	if withFolder != 3 {
		t.Fatalf("expected 3 tracks attached to the scan folder, got %d", withFolder)
	}
}

// A scan logs per-folder and per-phase milestones plus periodic progress, so
// the per-execution task log is informative instead of going silent between
// "starting" and "complete".
func TestScannerLogsProgress(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
		"Artist/Album/03.mp3",
	})
	folder := seedFolder(dir, nil)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true, Log: log}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{
		"folder scan planned",
		"scan plan",
		"reconciling folder",
		"scanning song",
		"indexing song",
		"folder reconciled",
		"running cleanup",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("scan log missing %q; full log:\n%s", want, out)
		}
	}
}

// A scan must not keep an album pointed at a cover file that no longer
// qualifies as front art — e.g. a back scan an earlier scan wrongly picked, or
// a cover file that has since been deleted.
func TestScannerRefreshesStaleAlbumCoverPath(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/cover.jpg",
	})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var album model.Album
	if err := st.DB().First(&album).Error; err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Artist/Album/cover.jpg")
	if album.CoverPath != want {
		t.Fatalf("CoverPath = %q, want %q", album.CoverPath, want)
	}

	// Simulate the state the old loose matcher left behind: the album points at
	// the sleeve's back scan.
	back := filepath.Join(dir, "Artist/Album/Back Cover.jpg")
	if err := os.WriteFile(back, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().Model(&model.Album{}).
		Where("id = ?", album.ID).
		Update("cover_path", back).Error; err != nil {
		t.Fatalf("seed stale cover path: %v", err)
	}

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().First(&album, album.ID).Error; err != nil {
		t.Fatal(err)
	}
	if album.CoverPath == back {
		t.Fatal("album still points at the back cover after a rescan")
	}
	if album.CoverPath != want {
		t.Errorf("CoverPath = %q, want %q", album.CoverPath, want)
	}
}

func TestScannerCleanupOrphans(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
	})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	_, _ = s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})

	_ = os.Remove(filepath.Join(dir, "Artist/Album/02.mp3"))
	_, _ = s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})

	var trackCount int64
	st.DB().Model(&model.Track{}).Count(&trackCount)
	if trackCount != 1 {
		t.Fatalf("expected 1 track after cleanup, got %d", trackCount)
	}
}

func TestScannerIncrementalSkipsUnchanged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	stats1, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if stats1.TracksProcessed != 1 {
		t.Fatalf("first scan: expected 1 processed, got %d", stats1.TracksProcessed)
	}
	stats2, _ := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false})
	if stats2.TracksProcessed != 0 {
		t.Fatalf("incremental scan: expected 0 processed, got %d", stats2.TracksProcessed)
	}
}

func TestScannerMultipleLibraries(t *testing.T) {
	st := testScanStore(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	createTestFiles(t, dir1, []string{"A/A/01.mp3"})
	createTestFiles(t, dir2, []string{"B/B/01.flac"})
	folderA := seedFolder(dir1, nil)
	folderB := seedFolder(dir2, nil)

	s := newScanner(t, st, fakeTagReader{}, folderA, folderB)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var aCount, bCount int64
	st.DB().Model(&model.Track{}).Where("scan_folder = ?", folderA.Name).Count(&aCount)
	st.DB().Model(&model.Track{}).Where("scan_folder = ?", folderB.Name).Count(&bCount)
	if aCount != 1 || bCount != 1 {
		t.Fatalf("expected one track per library, got A=%d B=%d", aCount, bCount)
	}
}

type multiTagReader struct{}

func (multiTagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }
func (multiTagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{"Artist A", "Artist B"},
		AlbumArtist: []string{"Artist A"},
		Album:       "Album",
		Genre:       []string{"Rock", "Jazz"},
		Duration:    180 * time.Second,
	}, nil
}

func TestScannerKeepsAllTagValues(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Album/01.mp3"})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, multiTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var artistCount int64
	st.DB().Model(&model.Artist{}).Count(&artistCount)
	if artistCount != 2 {
		t.Fatalf("expected 2 artists (multi-value kept as-is), got %d", artistCount)
	}
	var genreCount int64
	st.DB().Model(&model.Genre{}).Count(&genreCount)
	if genreCount != 2 {
		t.Fatalf("expected 2 genres, got %d", genreCount)
	}
}

// A library root that is gone (an unmounted share) must fail the scan. Walking
// it yields zero files and no error, which store.Cleanup would read as "the user
// deleted their whole library" and act on.
func TestScanFailsWhenTheLibraryRootIsMissing(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err == nil {
		t.Fatal("expected an error when the library root is gone")
	}

	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("the indexed track must survive an unreadable root, got %d rows", count)
	}
}

// The mountpoint case: the directory exists and is empty, so it stats fine. The
// only evidence that this is not a deletion is that the DB still holds tracks
// for the library.
func TestScanFailsWhenTheLibraryRootIsEmptyButTracksAreIndexed(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(dir, "Artist")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err == nil {
		t.Fatal("expected an error when a library with indexed tracks holds no audio files")
	}

	var count int64
	st.DB().Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("the indexed track must survive, got %d rows", count)
	}
}

// The guard must not block the legitimate empty case: a freshly added library
// with nothing in it yet.
func TestScanAllowsAnEmptyLibraryWithNothingIndexed(t *testing.T) {
	st := testScanStore(t)
	folder := seedFolder(t.TempDir(), nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatalf("an empty library with no indexed tracks must scan cleanly: %v", err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected 0 tracks processed, got %d", stats.TracksProcessed)
	}
}

type recordProgress struct {
	mu     sync.Mutex
	total  int64
	done   int64
	stages []string
}

func (r *recordProgress) SetTotal(t int64) { r.mu.Lock(); r.total = t; r.mu.Unlock() }
func (r *recordProgress) Inc(d int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done += d
	return r.done
}
func (r *recordProgress) SetStage(s string) {
	r.mu.Lock()
	r.stages = append(r.stages, s)
	r.mu.Unlock()
}

func (r *recordProgress) hasStagePrefix(p string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.stages {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// Every indexed track carries the name of the scan folder it was walked under
// and its lowercase extension — the two markers library scoping matches on.
func TestScanStampsScanFolderAndSuffix(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01-track.mp3",
		"Artist/Album/02-track.FLAC",
	})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var tracks []model.Track
	if err := st.DB().Order("filename").Find(&tracks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
	for _, tr := range tracks {
		if tr.ScanFolder != folder.Name {
			t.Errorf("%s: ScanFolder = %q, want %q", tr.Filename, tr.ScanFolder, folder.Name)
		}
	}
	if tracks[0].Suffix != "mp3" || tracks[1].Suffix != "flac" {
		t.Fatalf("suffixes = %q, %q; want mp3, flac (lowercased)", tracks[0].Suffix, tracks[1].Suffix)
	}
}

// An incremental scan reads no tags for unchanged files, so the stamp has to
// come from the pass that touches every walked file. This is what heals a
// renamed scan folder without a full scan.
func TestIncrementalScanRestampsScanFolder(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	// Simulate the row still carrying the folder's previous name.
	if err := st.DB().Model(&model.Track{}).Where("1 = 1").Update("scan_folder", "Old Name").Error; err != nil {
		t.Fatal(err)
	}

	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected the unchanged file not to be re-read, got %d processed", stats.TracksProcessed)
	}
	var got model.Track
	if err := st.DB().First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ScanFolder != folder.Name {
		t.Fatalf("ScanFolder = %q, want %q after an incremental scan", got.ScanFolder, folder.Name)
	}
}

func TestScannerReportsProgress(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Artist/Album/01.mp3",
		"Artist/Album/02.mp3",
		"Artist/Album/03.mp3",
	})
	folder := seedFolder(dir, nil)

	s := newScanner(t, st, fakeTagReader{}, folder)
	rec := &recordProgress{}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true, Progress: rec}); err != nil {
		t.Fatal(err)
	}

	// 3 files × 2 passes (read + save).
	if rec.total != 6 {
		t.Fatalf("SetTotal = %d, want 6", rec.total)
	}
	if rec.done != 6 {
		t.Fatalf("final done = %d, want 6", rec.done)
	}
	if !rec.hasStagePrefix("Extracting metadata: Artist/Album/") {
		t.Fatalf("no read-pass stage with a relative path; stages=%v", rec.stages)
	}
	if !rec.hasStagePrefix("Saving: Artist/Album/") {
		t.Fatalf("no save-pass stage with a relative path; stages=%v", rec.stages)
	}
	if !rec.hasStagePrefix("Cleaning up") {
		t.Fatalf("no cleanup stage; stages=%v", rec.stages)
	}
}

// A file reachable from two scan folders must belong to the same one whatever
// kind of scan ran last: the last folder to walk it, in name order. Two sibling
// folders reaching one directory through symlinks is the case a nested-path
// check cannot see, because the walker records the resolved path.
func TestScanFolderOwnershipIsStableAcrossScanKinds(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	createTestFiles(t, filepath.Join(base, "real"), []string{"Album/01.mp3"})
	rootA := filepath.Join(base, "a")
	rootB := filepath.Join(base, "b")
	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(base, "real"), filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
	}
	folderA := seedFolder(rootA, nil)
	folderB := seedFolder(rootB, nil)

	s := newScanner(t, st, fakeTagReader{}, folderA, folderB)
	for _, full := range []bool{true, false, true} {
		if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: full}); err != nil {
			t.Fatal(err)
		}
		var tracks []model.Track
		if err := st.DB().Find(&tracks).Error; err != nil {
			t.Fatal(err)
		}
		if len(tracks) != 1 {
			t.Fatalf("full=%v: expected the shared file indexed once, got %d rows", full, len(tracks))
		}
		if tracks[0].ScanFolder != folderB.Name {
			t.Fatalf("full=%v: ScanFolder = %q, want %q (last folder in name order)", full, tracks[0].ScanFolder, folderB.Name)
		}
	}
}

// With no scan folder configured a scan does nothing at all — in particular it
// must not reach Cleanup, which would sweep every indexed track.
func TestScanWithNoFoldersSweepsNothing(t *testing.T) {
	st := testScanStore(t)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	st.DB().Create(&album)
	st.DB().Create(&model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/gone/1.mp3", ScanFolder: "Gone"})

	stats, err := newScanner(t, st, fakeTagReader{}).Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 0 {
		t.Fatalf("expected nothing processed, got %d", stats.TracksProcessed)
	}
	var n int64
	st.DB().Model(&model.Track{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected the indexed track to survive, got %d rows", n)
	}
}

// Removing a folder from the config is the deliberate act that removes its
// tracks: nothing walks them any more, so the next scan's Cleanup sweeps them.
func TestScanSweepsTracksOfARemovedFolder(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	dirA, dirB := filepath.Join(base, "a"), filepath.Join(base, "b")
	createTestFiles(t, dirA, []string{"Artist/Album/01.mp3"})
	createTestFiles(t, dirB, []string{"Artist/Other/01.mp3"})
	folderA, folderB := seedFolder(dirA, nil), seedFolder(dirB, nil)

	if _, err := newScanner(t, st, fakeTagReader{}, folderA, folderB).Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	// The config now lists only A.
	if _, err := newScanner(t, st, fakeTagReader{}, folderA).Scan(context.Background(), scanner.ScanOptions{}); err != nil {
		t.Fatal(err)
	}

	var folders []string
	st.DB().Model(&model.Track{}).Order("scan_folder").Pluck("scan_folder", &folders)
	if len(folders) != 1 || folders[0] != folderA.Name {
		t.Fatalf("remaining tracks belong to %v, want only %q", folders, folderA.Name)
	}
}

// Pointing a scan folder at a new path must not wipe anything: the old paths
// vanish, the new ones appear, and track continuity re-links the rows — so the
// track id, and with it stars, playlists and play history, survives.
func TestScanFolderMovedToANewPathKeepsItsRows(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	oldRoot, newRoot := filepath.Join(base, "old"), filepath.Join(base, "new")
	createTestFiles(t, oldRoot, []string{"Artist/Album/01.mp3"})
	folder := scanfolder.Folder{Name: "Music", Path: oldRoot, FollowSymlinks: true}

	if _, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var before model.Track
	if err := st.DB().First(&before).Error; err != nil {
		t.Fatal(err)
	}
	st.DB().Create(&model.StarredItem{Owner: "admin", ItemType: "track", ItemID: before.ID})

	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	folder.Path = newRoot
	if _, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{}); err != nil {
		t.Fatal(err)
	}

	var after []model.Track
	st.DB().Find(&after)
	if len(after) != 1 || after[0].ID != before.ID {
		t.Fatalf("expected the same row re-linked, got %+v (was id %d)", after, before.ID)
	}
	if after[0].FilePath != filepath.Join(newRoot, "Artist/Album/01.mp3") {
		t.Fatalf("FilePath = %q, want the new location", after[0].FilePath)
	}
	var stars int64
	st.DB().Model(&model.StarredItem{}).Where("item_type = 'track' AND item_id = ?", before.ID).Count(&stars)
	if stars != 1 {
		t.Fatalf("expected the star to survive the move, got %d", stars)
	}
}

// A scan folder whose ROOT is itself a symlink must be refused before any
// write, not "succeed" with zero files: filepath.WalkDir does not descend a
// symlink root, and the follow-symlinks walk marks the resolved root seen and
// then skips it when it meets the root symlink.
func TestScanRefusesASymlinkedRoot(t *testing.T) {
	st := testScanStore(t)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	st.DB().Create(&album)
	st.DB().Create(&model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/anywhere/1.mp3", ScanFolder: "Music"})

	base := t.TempDir()
	real := filepath.Join(base, "real")
	createTestFiles(t, real, []string{"Artist/Album/01.mp3"})
	link := filepath.Join(base, "music")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}
	folder := scanfolder.Folder{Name: "Music", Path: link, FollowSymlinks: true}

	_, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{IsFull: true})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("err = %v, want it to mention 'symbolic link'", err)
	}

	var n int64
	st.DB().Model(&model.Track{}).Count(&n)
	if n != 1 {
		t.Fatalf("the pre-existing track must survive (the abort happens before any write), got %d rows", n)
	}
}

// The editor's post-save re-index must refuse the same roots the scan above
// refuses. Otherwise a folder the scanner will never walk gets indexed one edit
// at a time, under a spelling that stops resolving the day the operator applies
// the documented workaround (point Path at the real directory) — and those rows
// are then swept with their stars, playlist entries and play history.
func TestRescanPathsRefusesASymlinkedRoot(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	real := filepath.Join(base, "real")
	createTestFiles(t, real, []string{"Artist/Album/01.mp3"})
	link := filepath.Join(base, "music")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlinks on this platform: %v", err)
	}
	folder := scanfolder.Folder{Name: "Music", Path: link, FollowSymlinks: true}

	s := newScanner(t, st, fakeTagReader{}, folder)
	_, err := s.RescanPaths(context.Background(), folder.Name, []string{filepath.Join(link, "Artist/Album/01.mp3")})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("err = %v, want it to mention 'symbolic link'", err)
	}

	var n int64
	st.DB().Model(&model.Track{}).Count(&n)
	if n != 0 {
		t.Fatalf("a folder the scan refuses must not be indexed piecemeal, got %d rows", n)
	}
}

// The same guard's other half: a root that is not there — a share that has not
// mounted — is not re-indexed either.
func TestRescanPathsRefusesAnUnavailableRoot(t *testing.T) {
	st := testScanStore(t)
	missing := filepath.Join(t.TempDir(), "not-mounted")
	folder := scanfolder.Folder{Name: "Music", Path: missing}

	s := newScanner(t, st, fakeTagReader{}, folder)
	_, err := s.RescanPaths(context.Background(), folder.Name, []string{filepath.Join(missing, "Artist/Album/01.mp3")})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err = %v, want it to mention 'unavailable'", err)
	}

	var n int64
	st.DB().Model(&model.Track{}).Count(&n)
	if n != 0 {
		t.Fatalf("a folder the scan refuses must not be indexed piecemeal, got %d rows", n)
	}
}

// The re-link needs the old file to be GONE — that is what tells a move from a
// copy. Re-pointing a folder at a COPY while the original still exists therefore
// does not keep the rows: new ones are created and the old ones are swept, with
// their stars. Pinned so that changing it (treating rows no folder walks any more
// as re-link candidates) is a deliberate act; the docs state the condition.
func TestScanFolderRepointedAtACopyDoesNotKeepItsRows(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	oldRoot, newRoot := filepath.Join(base, "old"), filepath.Join(base, "new")
	createTestFiles(t, oldRoot, []string{"Artist/Album/01.mp3"})
	folder := scanfolder.Folder{Name: "Music", Path: oldRoot, FollowSymlinks: true}

	if _, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var before model.Track
	if err := st.DB().First(&before).Error; err != nil {
		t.Fatal(err)
	}
	st.DB().Create(&model.StarredItem{Owner: "admin", ItemType: "track", ItemID: before.ID})

	// Copy the tree to the new root, keeping the old one in place: the shape of
	// "rsync to a new disk, repoint Path, verify, delete the old copy later".
	createTestFiles(t, newRoot, []string{"Artist/Album/01.mp3"})
	folder.Path = newRoot
	if _, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{}); err != nil {
		t.Fatal(err)
	}

	var after []model.Track
	st.DB().Find(&after)
	if len(after) != 1 {
		t.Fatalf("expected exactly one track row, got %d: %+v", len(after), after)
	}
	if after[0].ID == before.ID {
		t.Fatalf("expected a NEW row (the old file is still on disk, so nothing is re-linked), got the same id %d", after[0].ID)
	}
	if after[0].FilePath != filepath.Join(newRoot, "Artist/Album/01.mp3") {
		t.Fatalf("FilePath = %q, want the new location", after[0].FilePath)
	}
	var stars int64
	st.DB().Model(&model.StarredItem{}).Where("item_type = 'track' AND item_id = ?", before.ID).Count(&stars)
	if stars != 0 {
		t.Fatalf("expected the star on the swept old row to be gone, got %d", stars)
	}
}

// The empty-walk guard must still trip when the folder was renamed in the config
// since its rows were stamped: the rows are found under the root's path range.
func TestScanEmptyWalkGuardSurvivesARename(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	if _, err := newScanner(t, st, fakeTagReader{}, scanfolder.Folder{Name: "Old Name", Path: dir, FollowSymlinks: true}).
		Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	// The share drops to a bare mountpoint AND the folder is renamed in config.
	if err := os.RemoveAll(filepath.Join(dir, "Artist")); err != nil {
		t.Fatal(err)
	}
	_, err := newScanner(t, st, fakeTagReader{}, scanfolder.Folder{Name: "New Name", Path: dir, FollowSymlinks: true}).
		Scan(context.Background(), scanner.ScanOptions{})
	if err == nil || !strings.Contains(err.Error(), "refusing to delete") {
		t.Fatalf("err = %v, want the empty-walk guard to refuse", err)
	}
	var n int64
	st.DB().Model(&model.Track{}).Count(&n)
	if n != 1 {
		t.Fatalf("the guard must leave the track in place, got %d rows", n)
	}
}

// Guard 2's remedy ("remove it from ScanFolders … the next scan then removes
// those N tracks") is false when this is the LAST scan folder configured:
// with none left, Scan returns before Cleanup ever runs, so nothing would
// actually be removed. The message must say so instead of the normal remedy.
func TestScanEmptyWalkGuardMessageWarnsWhenItIsTheOnlyFolder(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Artist/Album/01.mp3"})
	folder := seedFolder(dir, nil)
	if _, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "Artist")); err != nil {
		t.Fatal(err)
	}

	_, err := newScanner(t, st, fakeTagReader{}, folder).Scan(context.Background(), scanner.ScanOptions{})
	if err == nil || !strings.Contains(err.Error(), "refusing to delete") {
		t.Fatalf("err = %v, want the empty-walk guard to refuse", err)
	}
	if !strings.Contains(err.Error(), "only scan folder configured") {
		t.Fatalf("err = %v, want it to say this is the only scan folder configured", err)
	}
}

// The same guard with a SECOND folder still configured: the normal removal
// remedy stays true (a scan still reaches Cleanup), so the message must not
// carry the only-folder caveat.
func TestScanEmptyWalkGuardMessageOmitsTheOnlyFolderCaveatWithOthersConfigured(t *testing.T) {
	st := testScanStore(t)
	base := t.TempDir()
	dirA, dirB := filepath.Join(base, "a"), filepath.Join(base, "b")
	createTestFiles(t, dirA, []string{"Artist/Album/01.mp3"})
	createTestFiles(t, dirB, []string{"Artist/Other/01.mp3"})
	folderA, folderB := seedFolder(dirA, nil), seedFolder(dirB, nil)
	if _, err := newScanner(t, st, fakeTagReader{}, folderA, folderB).Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dirA, "Artist")); err != nil {
		t.Fatal(err)
	}

	_, err := newScanner(t, st, fakeTagReader{}, folderA, folderB).Scan(context.Background(), scanner.ScanOptions{})
	if err == nil || !strings.Contains(err.Error(), "refusing to delete") {
		t.Fatalf("err = %v, want the empty-walk guard to refuse", err)
	}
	if strings.Contains(err.Error(), "only scan folder configured") {
		t.Fatalf("err = %v, must not claim to be the only scan folder configured", err)
	}
	if !strings.Contains(err.Error(), "remove it from ScanFolders") {
		t.Fatalf("err = %v, want the normal removal remedy", err)
	}
}
