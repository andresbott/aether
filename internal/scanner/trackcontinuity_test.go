package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

// movingTagReader reports tags per absolute path, and the tests re-point a path
// at the same title when they move a file — which is what happens on disk, where
// the tags travel with the bytes. fakeTagReader (scanner_test.go) derives Title
// from the filename, so it would report a *different* title after a rename and
// defeat the very thing under test.
type movingTagReader struct {
	titles   map[string]string
	duration time.Duration // zero means 180s
}

func (r *movingTagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r *movingTagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	dur := r.duration
	if dur == 0 {
		dur = 180 * time.Second
	}
	return tags.Metadata{
		Title:       r.titles[absPath],
		Artist:      []string{"Apocalyptica"},
		AlbumArtist: []string{"Apocalyptica"},
		Album:       "Cult",
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    dur,
		Bitrate:     320,
	}, nil
}

// moveFile renames a file on disk and carries its tags to the new path, the way
// a file manager or a tagger would. os.Rename preserves the mod time.
func moveFile(t *testing.T, r *movingTagReader, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	r.titles[to] = r.titles[from]
	delete(r.titles, from)
}

// theOnlyTrack fails unless the DB holds exactly one track row, and returns it.
func theOnlyTrack(t *testing.T, st *store.Store) model.Track {
	t.Helper()
	var tracks []model.Track
	if err := st.DB().Find(&tracks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected exactly 1 track row, got %d: %+v", len(tracks), tracks)
	}
	return tracks[0]
}

// The core promise: a file that moves keeps its row, so everything keyed on
// tracks.id survives — playlist memberships, play history, stars and the queue,
// all of which store.DeleteOrphanedAggregates hard-deletes when the id dies.
func TestScanKeepsTheTrackIDWhenAFileMoves(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// Reorganised: a new folder AND a new filename, bytes untouched.
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01 - Path Of Glory.mp3")
	moveFile(t, reader, src, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("track id changed on a move: was %d, now %d", before.ID, after.ID)
	}
	if after.FilePath != dst {
		t.Fatalf("FilePath = %q, want the new path %q", after.FilePath, dst)
	}
	if after.Filename != "01 - Path Of Glory.mp3" {
		t.Fatalf("Filename = %q, want the new basename", after.Filename)
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at changed on a move (%v -> %v)", before.CreatedAt, after.CreatedAt)
	}
}

// A rename in place: same directory, new basename. The basename is deliberately
// not part of the proof, so this is provable for the same reason a move is.
func TestScanKeepsTheTrackIDWhenAFileIsRenamed(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/track1.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/track1.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	dst := filepath.Join(dir, "Apocalyptica/Cult/01 - Path Of Glory.mp3")
	moveFile(t, reader, src, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("track id changed on a rename: was %d, now %d", before.ID, after.ID)
	}
	if after.FilePath != dst || after.Filename != "01 - Path Of Glory.mp3" {
		t.Fatalf("row did not follow the rename: path %q, filename %q", after.FilePath, after.Filename)
	}
}

// A copy is not a move. The original file is still on disk, so its row is alive
// and must keep its own path; the copy is a new track.
func TestScanDoesNotRelinkACopiedFile(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	dst := filepath.Join(dir, "Compilations/Best Of/01.mp3")
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("fake"), 0o600); err != nil { // same bytes as createTestFiles
		t.Fatal(err)
	}
	reader.titles[dst] = reader.titles[src]

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var tracks []model.Track
	if err := st.DB().Order("id").Find(&tracks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 rows (original + copy), got %d: %+v", len(tracks), tracks)
	}
	if tracks[0].ID != before.ID || tracks[0].FilePath != src {
		t.Fatalf("the original row must keep its id and its path, got %+v", tracks[0])
	}
}

// A move with a differing duration must not be re-linked: duration is part of
// the proof. This tests the durationsAgree reject path.
func TestScanDoesNotRelinkWhenTheDurationDiffers(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// Move the file but report a different duration (300s vs 180s).
	reader.duration = 300 * time.Second
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01 - Path Of Glory.mp3")
	moveFile(t, reader, src, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID == before.ID {
		t.Fatal("a move with a differing duration must not be re-linked: duration is part of the proof")
	}
}

// A copy on an incremental scan must not be re-linked: the os.Stat guard is the
// only defence when inBatch does not cover the original path. This test verifies
// that the stat check genuinely runs and prevents the copy from stealing the
// original's identity.
func TestScanDoesNotRelinkACopiedFileOnAnIncrementalScan(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	// An old, whole-second mod time so FilterChanged provably filters src out of
	// the incremental batch below. Without it the test could pass vacuously: a
	// src that re-enters the batch is covered by inBatch and never reaches the
	// os.Stat guard this test exists to exercise.
	stampModTime(t, src, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// Copy the file to a second path with the same bytes and title.
	dst := filepath.Join(dir, "Compilations/Best Of/01.mp3")
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader.titles[dst] = reader.titles[src]

	// Incremental scan: only the new path is in results, so inBatch does not
	// cover src. The os.Stat check is the sole defence against re-linking.
	stats, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TracksProcessed != 1 {
		t.Fatalf("expected only the new path in the incremental batch (TracksProcessed=1), got %d — "+
			"src re-entered the batch, so inBatch covers it and the os.Stat guard is untested",
			stats.TracksProcessed)
	}

	var tracks []model.Track
	if err := st.DB().Order("id").Find(&tracks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 rows (original + copy), got %d: %+v", len(tracks), tracks)
	}
	if tracks[0].ID != before.ID || tracks[0].FilePath != src {
		t.Fatalf("the original row must keep its id and its path, got %+v", tracks[0])
	}
}

// An unreadable directory is not a deletion. planTrackContinuity narrows its
// "the old file is gone" test to fs.ErrNotExist exactly so an EACCES stat cannot
// put a live row into `vanished` — and since Scan's preflight only guards library
// *roots*, that narrowing is the last defence for a subtree that became
// unreachable inside a root that is present (a permission change, a per-directory
// mount that went away). Widening the check to "any stat error means gone" passes
// every other test in the suite; this is the one that fails.
func TestScanDoesNotRelinkWhenTheOldDirectoryIsUnreadable(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// The file stays on disk; only the way to it is taken away, so stat-ing it
	// fails with EACCES instead of ENOENT.
	srcDir := filepath.Dir(src)
	if err := os.Chmod(srcDir, 0o000); err != nil {
		t.Fatal(err)
	}
	// Registered after t.TempDir's own cleanup and therefore run before it, which
	// is the only reason the temp tree can still be removed.
	t.Cleanup(func() { _ = os.Chmod(srcDir, 0o750) })
	if _, err := os.Stat(src); err == nil {
		t.Skip("this process stats through mode 0000 (running as root); the EACCES path is unreachable here")
	}

	// A byte-identical file with the same title elsewhere in the same library:
	// every part of the proof except the stat says "this is where 01.mp3 moved
	// to". It also keeps the walk non-empty, so the sweep guard does not fire
	// first and mask the case under test.
	dst := filepath.Join(dir, "Compilations/Best Of/01.mp3")
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader.titles[dst] = "Path Of Glory"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var moved model.Track
	if err := st.DB().Where("file_path = ?", dst).First(&moved).Error; err != nil {
		t.Fatal(err)
	}
	if moved.ID == before.ID {
		t.Fatalf("row %d was re-linked onto %q although its old file only became unreadable rather "+
			"than deleted: an EACCES stat is not evidence of absence", before.ID, dst)
	}
}

// A move where the duration differs by exactly 1 second (within the tolerance)
// must still be re-linked. This tests the durationsAgree accept path: production
// uses tags.NewFallbackReader(taglib, ffprobe), and the same file read by
// different readers can round differently.
func TestScanRelinksWhenTheDurationDiffersBy1Second(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// Move the file and report a duration 1 second longer (within tolerance).
	reader.duration = 181 * time.Second
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01 - Path Of Glory.mp3")
	moveFile(t, reader, src, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("track id changed on a 1-second duration difference (within tolerance): was %d, now %d", before.ID, after.ID)
	}
	if after.FilePath != dst {
		t.Fatalf("FilePath = %q, want the new path %q", after.FilePath, dst)
	}
}

// stampModTime pins a file's mod time so the tiebreak is deterministic instead
// of depending on how the clock fell during fixture creation.
func stampModTime(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

// Two byte-identical files with the same title: one is deleted and one is moved.
// The fingerprint cannot say which row the surviving file is, and with equal mod
// times nothing else can either — so nothing is re-linked. Merging two tracks'
// history is worse than losing one's.
func TestScanDoesNotRelinkAnAmbiguousFingerprint(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3", "Apocalyptica/Cult/02.mp3"})
	seedLibrary(t, st, dir, nil)

	one := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	two := filepath.Join(dir, "Apocalyptica/Cult/02.mp3")
	same := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stampModTime(t, one, same)
	stampModTime(t, two, same)

	// Same title on both: duplicates of one track, which is what makes the
	// fingerprint ambiguous.
	reader := &movingTagReader{titles: map[string]string{one: "Beyond Time", two: "Beyond Time"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var seeded []model.Track
	if err := st.DB().Order("id").Find(&seeded).Error; err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 2 {
		t.Fatalf("expected 2 seeded rows, got %d", len(seeded))
	}

	if err := os.Remove(one); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/02.mp3")
	moveFile(t, reader, two, dst)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	for _, before := range seeded {
		if after.ID == before.ID {
			t.Fatalf("row %d was re-linked from an ambiguous fingerprint", before.ID)
		}
	}
}

// Same ambiguity, except the two files have different mod times and the moved
// one kept its own (os.Rename preserves it). That singles out exactly one
// vanished row, so the move is provable after all.
func TestScanRelinksTheVanishedRowWhoseModTimeMatches(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3", "Apocalyptica/Cult/02.mp3"})
	seedLibrary(t, st, dir, nil)

	one := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	two := filepath.Join(dir, "Apocalyptica/Cult/02.mp3")
	stampModTime(t, one, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	stampModTime(t, two, time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC))

	reader := &movingTagReader{titles: map[string]string{one: "Beyond Time", two: "Beyond Time"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var twoRow model.Track
	if err := st.DB().Where("file_path = ?", two).First(&twoRow).Error; err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(one); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/02.mp3")
	moveFile(t, reader, two, dst)

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != twoRow.ID {
		t.Fatalf("expected the row whose mod time matches (%d) to be re-linked, got %d", twoRow.ID, after.ID)
	}
	if after.FilePath != dst {
		t.Fatalf("FilePath = %q, want %q", after.FilePath, dst)
	}
}

// Mirrors TestScanRelinksTheVanishedRowWhoseModTimeMatches: one vanished row,
// several new files with the same fingerprint, but only one has the matching
// mod time. This exercises the onlyFileWithModTime tiebreak branch.
func TestScanRelinksTheNewFileWhoseModTimeMatches(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	stampedTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stampModTime(t, src, stampedTime)

	reader := &movingTagReader{titles: map[string]string{src: "Beyond Time"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// Move the file to a new path.
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01.mp3")
	moveFile(t, reader, src, dst)

	// Create a second new file with the same 4-byte body and same title, but a
	// clearly different mod time. Both paths are unclaimed and share the
	// fingerprint; only one vanished row exists; the mod time must pick the moved
	// file.
	other := filepath.Join(dir, "Apocalyptica/Cult (2000)/duplicate.mp3")
	if err := os.WriteFile(other, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	stampModTime(t, other, time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC))
	reader.titles[other] = "Beyond Time"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var tracks []model.Track
	if err := st.DB().Order("id").Find(&tracks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected exactly 2 rows (relinked + new), got %d: %+v", len(tracks), tracks)
	}

	// The row whose file_path is dst must have kept the original id.
	var movedRow *model.Track
	var otherRow *model.Track
	for i := range tracks {
		if tracks[i].FilePath == dst {
			movedRow = &tracks[i]
		} else {
			otherRow = &tracks[i]
		}
	}
	if movedRow == nil || otherRow == nil {
		t.Fatalf("expected one row at %q and one at %q, got %+v", dst, other, tracks)
	}
	if movedRow.ID != before.ID {
		t.Fatalf("expected the file whose mod time matches to be re-linked (id %d), got %d", before.ID, movedRow.ID)
	}
	if otherRow.ID == before.ID {
		t.Fatalf("the other file must have a different id, got %d", otherRow.ID)
	}
}

// namedLibrary is seedLibrary with the name spelled out, because ListLibraries
// orders by name and the test below needs the unavailable library to sort *after*
// the healthy one — that ordering is the whole bug.
func namedLibrary(t *testing.T, s *store.Store, name, path string) *model.Library {
	t.Helper()
	lib := &model.Library{Name: name, Path: path, FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	return lib
}

// An unavailable library must not have its rows harvested by an earlier
// library's re-link pass. Scan returns on the first failing guard, but the
// candidate pool is deliberately cross-library, so before Scan was split into
// preflight + reconcile every row of an unmounted "Zarchive" stat-ed ENOENT and
// became a move candidate while "Music" was still being reconciled — a single
// byte-identical new file was enough to move an unreachable library's stars,
// playlists, history and library_id onto it, and the guard then failed the scan
// too late to undo any of it.
func TestScanValidatesEveryLibraryBeforeReconcilingAny(t *testing.T) {
	st := testScanStore(t)
	musicDir := t.TempDir()
	archiveDir := t.TempDir()
	createTestFiles(t, musicDir, []string{"Apocalyptica/Cult/01.mp3"})
	createTestFiles(t, archiveDir, []string{"Mirror/Cult/01.mp3"})
	namedLibrary(t, st, "Music", musicDir)
	archiveLib := namedLibrary(t, st, "Zarchive", archiveDir)

	music := filepath.Join(musicDir, "Apocalyptica/Cult/01.mp3")
	archived := filepath.Join(archiveDir, "Mirror/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{
		music:    "Beyond Time",
		archived: "Path Of Glory",
	}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var before model.Track
	if err := st.DB().Where("file_path = ?", archived).First(&before).Error; err != nil {
		t.Fatal(err)
	}

	// The archive drive goes away, and a new file lands in Music that is
	// byte-identical to the archived one and carries its title: everything the
	// proof asks for except that the old file is not actually gone.
	if err := os.RemoveAll(archiveDir); err != nil {
		t.Fatal(err)
	}
	bait := filepath.Join(musicDir, "Apocalyptica/Cult/02.mp3")
	if err := os.WriteFile(bait, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader.titles[bait] = "Path Of Glory"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err == nil {
		t.Fatal("expected the scan to fail on the unavailable library")
	}

	var after model.Track
	if err := st.DB().First(&after, before.ID).Error; err != nil {
		t.Fatalf("the unavailable library's row must survive untouched: %v", err)
	}
	if after.FilePath != archived {
		t.Fatalf("row %d was re-linked to %q by an earlier library's pass; it belongs to a library "+
			"whose guard had not run yet", before.ID, after.FilePath)
	}
	if after.LibraryID != archiveLib.ID {
		t.Fatalf("LibraryID = %d, want the archive library %d", after.LibraryID, archiveLib.ID)
	}
	var count int64
	if err := st.DB().Model(&model.Track{}).Where("file_path = ?", bait).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("no library may be reconciled when a later one fails its guard, but %q was indexed", bait)
	}
}

// The whole point. Each of these four is hard-deleted by
// DeleteOrphanedAggregates when a track id dies (internal/store/scan_helpers.go)
// and none of it is recoverable or re-derivable — it is the user's own data.
func TestScanMoveKeepsPlaylistsStarsHistoryAndQueue(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Apocalyptica/Cult/01.mp3"})
	seedLibrary(t, st, dir, nil)

	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	reader := &movingTagReader{titles: map[string]string{src: "Path Of Glory"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	if _, err := st.CreatePlaylist("Mix", "alice", false, []uint{before.ID}); err != nil {
		t.Fatal(err)
	}
	if err := st.Star("alice", "track", before.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordPlay("alice", before.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := st.SavePlayQueue("alice", []uint{before.ID}, 0, 0, "test", time.Now()); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01.mp3")
	moveFile(t, reader, src, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("track id changed on a move: was %d, now %d", before.ID, after.ID)
	}

	for _, c := range []struct {
		table string
		where string
		args  []any
	}{
		{"playlist_tracks", "track_id = ?", []any{before.ID}},
		{"starred_items", "item_type = ? AND item_id = ?", []any{"track", before.ID}},
		{"play_histories", "track_id = ?", []any{before.ID}},
		{"play_queue_entries", "track_id = ?", []any{before.ID}},
	} {
		var n int64
		if err := st.DB().Table(c.table).Where(c.where, c.args...).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("%s: expected 1 row for track %d after the move, found %d", c.table, before.ID, n)
		}
	}
}

// Moving a file into another collection keeps the row and rewrites library_id.
// The source library keeps a second file so the move does not empty it — an
// emptied library is a different case, and the sweep guard (Task 6) stops the
// scan there on purpose.
func TestScanKeepsTheTrackIDWhenAFileMovesBetweenLibraries(t *testing.T) {
	st := testScanStore(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	createTestFiles(t, dirA, []string{"Apocalyptica/Cult/01.mp3", "Apocalyptica/Cult/02.mp3"})
	createTestFiles(t, dirB, []string{"Metallica/S&M/01.mp3"})
	seedLibrary(t, st, dirA, nil)
	libB := seedLibrary(t, st, dirB, nil)

	moving := filepath.Join(dirA, "Apocalyptica/Cult/01.mp3")
	stayingA := filepath.Join(dirA, "Apocalyptica/Cult/02.mp3")
	stayingB := filepath.Join(dirB, "Metallica/S&M/01.mp3")
	reader := &movingTagReader{titles: map[string]string{
		moving:   "Path Of Glory",
		stayingA: "Beyond Time",
		stayingB: "No Leaf Clover",
	}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	var before model.Track
	if err := st.DB().Where("file_path = ?", moving).First(&before).Error; err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dirB, "Apocalyptica/Cult/01.mp3")
	moveFile(t, reader, moving, dst)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var after model.Track
	if err := st.DB().Where("file_path = ?", dst).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID {
		t.Fatalf("track id changed on a cross-library move: was %d, now %d", before.ID, after.ID)
	}
	if after.LibraryID != libB.ID {
		t.Fatalf("LibraryID = %d, want the destination library %d", after.LibraryID, libB.ID)
	}
}

// A whole album folder moved: every track id survives, and so does the album
// row. The album part is a regression guard on the ordering inside reconcile —
// planAlbumContinuity counts an album's tracks by looking up the batch's paths,
// so re-linking has to happen first for a move never to look like a split.
func TestScanKeepsTrackAndAlbumIdentityWhenAWholeAlbumMoves(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
		"Apocalyptica/Cult/03.mp3",
	})
	seedLibrary(t, st, dir, nil)

	titles := map[string]string{}
	for i, name := range []string{"01.mp3", "02.mp3", "03.mp3"} {
		titles[filepath.Join(dir, "Apocalyptica/Cult", name)] = string(rune('A' + i))
	}
	reader := &movingTagReader{titles: titles}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	beforeIDs := map[string]uint{}
	var seeded []model.Track
	if err := st.DB().Find(&seeded).Error; err != nil {
		t.Fatal(err)
	}
	for _, track := range seeded {
		beforeIDs[track.Title] = track.ID
	}
	if len(beforeIDs) != 3 {
		t.Fatalf("expected 3 distinct titles, got %d", len(beforeIDs))
	}
	beforeAlbum := theOnlyAlbum(t, st)

	for _, name := range []string{"01.mp3", "02.mp3", "03.mp3"} {
		moveFile(t, reader,
			filepath.Join(dir, "Apocalyptica/Cult", name),
			filepath.Join(dir, "Music/Apocalyptica - Cult", name))
	}
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	var moved []model.Track
	if err := st.DB().Find(&moved).Error; err != nil {
		t.Fatal(err)
	}
	if len(moved) != 3 {
		t.Fatalf("expected 3 rows after the move, got %d", len(moved))
	}
	for _, track := range moved {
		if beforeIDs[track.Title] != track.ID {
			t.Errorf("%q: id changed from %d to %d", track.Title, beforeIDs[track.Title], track.ID)
		}
	}

	afterAlbum := theOnlyAlbum(t, st)
	if afterAlbum.ID != beforeAlbum.ID {
		t.Fatalf("album id changed when the album moved: was %d, now %d", beforeAlbum.ID, afterAlbum.ID)
	}
	if !afterAlbum.CreatedAt.Equal(beforeAlbum.CreatedAt) {
		t.Fatalf("album created_at changed (%v -> %v)", beforeAlbum.CreatedAt, afterAlbum.CreatedAt)
	}
}

// ----- Retagged moves: the audio-hash proof -----

// writeMP3WithTag writes a file libs/audiohash reads as an MP3: a real ID3v2
// header declaring tagLen bytes of tag body, then the audio payload.
//
// Varying tagLen is exactly what a tag edit does on disk — the tag region grows
// or shrinks and the audio after it is untouched — so two files written with the
// same payload and different tagLen have different sizes and the *same*
// metadata-invariant hash. That is the whole property under test, and using the
// real header format means these tests exercise libs/audiohash rather than a
// stand-in for it.
func writeMP3WithTag(t *testing.T, path string, tagLen int, payload string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	hdr := make([]byte, 10)
	copy(hdr, "ID3")
	hdr[3] = 3 // ID3v2.3
	// Bytes 6..9 are the tag size as a synchsafe integer (7 bits per byte).
	hdr[6] = byte((tagLen >> 21) & 0x7f)
	hdr[7] = byte((tagLen >> 14) & 0x7f)
	hdr[8] = byte((tagLen >> 7) & 0x7f)
	hdr[9] = byte(tagLen & 0x7f)

	body := make([]byte, 0, 10+tagLen+len(payload))
	body = append(body, hdr...)
	body = append(body, make([]byte, tagLen)...)
	body = append(body, payload...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The case the size-and-title proof cannot reach: a tagger fixed the tags and
// re-filed the track in one operation, so the path, the title AND the byte count
// all changed at once. Only the audio hash still connects the two ends — and it
// has to, because this is what Picard and beets do by default.
func TestScanRelinksARetaggedMoveByAudioHash(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "Apocaliptica/Cult/01.mp3")
	writeMP3WithTag(t, src, 20, "the-audio-payload")
	seedLibrary(t, st, dir, nil)

	reader := &movingTagReader{titles: map[string]string{src: "Path"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)
	if before.AudioHash == "" {
		t.Fatal("the scan must record an audio hash, or there is nothing for the proof to match on")
	}
	if err := st.Star("alice", "track", before.ID); err != nil {
		t.Fatal(err)
	}

	// The tagger rewrote the tags (bigger tag region, corrected title) and moved
	// the file into a corrected folder. The audio payload is byte-identical.
	dst := filepath.Join(dir, "Apocalyptica/Cult/01 - Path Of Glory.mp3")
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	writeMP3WithTag(t, dst, 120, "the-audio-payload")
	delete(reader.titles, src)
	reader.titles[dst] = "Path Of Glory"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("a retagged move must keep the row: id was %d, now %d", before.ID, after.ID)
	}
	if after.FilePath != dst {
		t.Fatalf("expected the row to carry the new path, got %q", after.FilePath)
	}
	if after.FileSize == before.FileSize {
		t.Fatal("the fixture is wrong: the retag must change the file size, or the size proof would have carried this")
	}
	if after.AudioHash != before.AudioHash {
		t.Fatalf("the audio hash must survive a tag edit: was %q, now %q", before.AudioHash, after.AudioHash)
	}

	var stars int64
	if err := st.DB().Table("starred_items").
		Where("item_type = ? AND item_id = ?", "track", before.ID).
		Count(&stars).Error; err != nil {
		t.Fatal(err)
	}
	if stars != 1 {
		t.Fatalf("expected the star to survive the retagged move, found %d rows", stars)
	}
}

// The hash is metadata-invariant, not content-invariant: when the audio itself
// differs, both proofs must decline. Re-encoding a track at another bitrate and
// re-filing it is a new file, not a move.
func TestScanDoesNotRelinkWhenTheAudioItselfChanged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	writeMP3WithTag(t, src, 20, "the-audio-payload")
	seedLibrary(t, st, dir, nil)

	reader := &movingTagReader{titles: map[string]string{src: "Path"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	dst := filepath.Join(dir, "Apocalyptica/Cult/01 - Path.mp3")
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	writeMP3WithTag(t, dst, 20, "a-different-audio-payload")
	delete(reader.titles, src)
	reader.titles[dst] = "Path"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID == before.ID {
		t.Fatal("different audio must not be re-linked onto the old row: the hash is metadata-invariant, not content-invariant")
	}
}

// A format libs/audiohash cannot read has no hash, so a retagged move of it
// keeps only the size-and-title proof — which a retag defeats. Locks in the
// declined tier and the graceful fallback: no hash must never mean no scan.
func TestScanDoesNotRelinkARetaggedMoveWithoutAHash(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "Apocalyptica/Cult/01.ogg")
	if err := os.MkdirAll(filepath.Dir(src), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("ogg-audio-payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	seedLibrary(t, st, dir, nil)

	reader := &movingTagReader{titles: map[string]string{src: "Path"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)
	if before.AudioHash != "" {
		t.Fatalf("an unsupported format must store no hash, got %q", before.AudioHash)
	}

	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01.ogg")
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("ogg-audio-payload-with-bigger-tags"), 0o600); err != nil {
		t.Fatal(err)
	}
	delete(reader.titles, src)
	reader.titles[dst] = "Path Of Glory"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID == before.ID {
		t.Fatal("without a hash there is no proof for a retagged move; it must fall back to insert plus delete")
	}
}

// A tag edit in place (no move) must leave the stored hash alone, because that
// stored value is what a *later* move will be proved with. This is the editor's
// own path: it writes tags and calls RescanPaths on what it wrote.
func TestRescanPathsKeepsTheAudioHashAcrossAnInPlaceRetag(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Apocalyptica/Cult/01.mp3")
	writeMP3WithTag(t, path, 20, "the-audio-payload")
	lib := seedLibrary(t, st, dir, nil)

	reader := &movingTagReader{titles: map[string]string{path: "Path"}}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyTrack(t, st)

	// The editor rewrites the tags of the same file: bigger tag region, same audio.
	writeMP3WithTag(t, path, 200, "the-audio-payload")
	reader.titles[path] = "Path Of Glory"
	if _, err := s.RescanPaths(context.Background(), lib.ID, []string{path}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID != before.ID {
		t.Fatalf("an in-place retag must not churn the row: id was %d, now %d", before.ID, after.ID)
	}
	if after.AudioHash != before.AudioHash {
		t.Fatalf("the hash must be unchanged by a tag edit: was %q, now %q", before.AudioHash, after.AudioHash)
	}
	if after.FileSize == before.FileSize {
		t.Fatal("the fixture is wrong: the retag must change the file size")
	}
}
