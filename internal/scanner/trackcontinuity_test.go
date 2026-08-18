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

// A move that also rewrote the tags changes the bytes, so file_size no longer
// matches and the proof fails by design. Locks in the declined tier: matching
// across a retag would rest on MBRecordingID, which identifies a recording
// rather than a file.
func TestScanDoesNotRelinkWhenTheFileSizeChanged(t *testing.T) {
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

	// Retag + re-file: the file is gone from the old path and the new one holds
	// more bytes, because the tags were rewritten on the way.
	dst := filepath.Join(dir, "Apocalyptica/Cult (2000)/01.mp3")
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("fake plus fresh tags"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader.titles[dst] = "Path Of Glory"

	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyTrack(t, st)
	if after.ID == before.ID {
		t.Fatal("a retagged move must not be re-linked: the proof rests on file_size")
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
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: false}); err != nil {
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
