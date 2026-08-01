package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func TestSavePlayQueueRoundTrip(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3", Title: "One"}
	t2 := model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/2.mp3", Title: "Two"}
	db.Create(&t1)
	db.Create(&t2)

	changed := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	if err := s.SavePlayQueue("admin", []uint{t1.ID, t2.ID}, 1, 42000, "test-client", changed); err != nil {
		t.Fatal(err)
	}

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("expected a saved queue, got nil")
	}
	if got := q.CurrentIndex; got != 1 {
		t.Fatalf("CurrentIndex = %d, want 1", got)
	}
	if got := q.PositionMs; got != 42000 {
		t.Fatalf("PositionMs = %d, want 42000", got)
	}
	if got := q.ChangedBy; got != "test-client" {
		t.Fatalf("ChangedBy = %q, want %q", got, "test-client")
	}
	if !q.Changed.Equal(changed) {
		t.Fatalf("Changed = %v, want %v", q.Changed, changed)
	}
	if len(q.Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(q.Tracks))
	}
	if q.Tracks[0].ID != t1.ID || q.Tracks[1].ID != t2.ID {
		t.Fatalf("track order = [%d %d], want [%d %d]", q.Tracks[0].ID, q.Tracks[1].ID, t1.ID, t2.ID)
	}
}

// The queue is per-owner and singular: saving again replaces it wholesale rather
// than appending a second queue.
func TestSavePlayQueueReplacesPrevious(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/2.mp3"}
	t3 := model.Track{AlbumID: album.ID, Filename: "3.mp3", FilePath: "/3.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	db.Create(&t3)

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SavePlayQueue("admin", []uint{t1.ID, t2.ID}, 0, 0, "c1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePlayQueue("admin", []uint{t3.ID}, 0, 5000, "c2", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var queues int64
	db.Model(&model.PlayQueue{}).Count(&queues)
	if queues != 1 {
		t.Fatalf("expected exactly 1 play_queues row, got %d", queues)
	}
	var entries int64
	db.Model(&model.PlayQueueEntry{}).Count(&entries)
	if entries != 1 {
		t.Fatalf("expected the old entries replaced, got %d rows", entries)
	}

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tracks) != 1 || q.Tracks[0].ID != t3.ID {
		t.Fatalf("expected only track %d in the queue, got %+v", t3.ID, q.Tracks)
	}
	if q.ChangedBy != "c2" {
		t.Fatalf("ChangedBy = %q, want c2", q.ChangedBy)
	}
}

// A queue that holds the same track twice must keep both slots — this is exactly
// why the current track is stored as an index and not as a track id.
func TestSavePlayQueueKeepsDuplicateTracks(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&tr)

	if err := s.SavePlayQueue("admin", []uint{tr.ID, tr.ID, tr.ID}, 2, 1000, "c", time.Now()); err != nil {
		t.Fatal(err)
	}
	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tracks) != 3 {
		t.Fatalf("len(Tracks) = %d, want 3 (duplicates preserved)", len(q.Tracks))
	}
	if q.CurrentIndex != 2 {
		t.Fatalf("CurrentIndex = %d, want 2", q.CurrentIndex)
	}
}

func TestGetPlayQueueAbsentReturnsNil(t *testing.T) {
	s := testStore(t)
	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatalf("expected no error for a missing queue, got %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil queue, got %+v", q)
	}
}

func TestClearPlayQueueRemovesQueueAndEntries(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&tr)
	if err := s.SavePlayQueue("admin", []uint{tr.ID}, 0, 0, "c", time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := s.ClearPlayQueue("admin"); err != nil {
		t.Fatal(err)
	}
	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if q != nil {
		t.Fatal("expected the queue gone after ClearPlayQueue")
	}
	var entries int64
	db.Model(&model.PlayQueueEntry{}).Count(&entries)
	if entries != 0 {
		t.Fatalf("expected no entry rows left, got %d", entries)
	}
}

// Clearing a queue that was never saved is a no-op, not an error: savePlayQueue
// with no ids is the spec's "clear" call and may arrive at any time.
func TestClearPlayQueueWithoutSavedQueue(t *testing.T) {
	s := testStore(t)
	if err := s.ClearPlayQueue("admin"); err != nil {
		t.Fatalf("expected no error clearing an absent queue, got %v", err)
	}
}

// A track deleted by a rescan must drop out of the queue rather than resurface as
// a nil entry, and the current index has to follow the tracks that remain.
func TestGetPlayQueueSkipsDeletedTracks(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/2.mp3"}
	t3 := model.Track{AlbumID: album.ID, Filename: "3.mp3", FilePath: "/3.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	db.Create(&t3)
	if err := s.SavePlayQueue("admin", []uint{t1.ID, t2.ID, t3.ID}, 2, 0, "c", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Drop the middle track, as a rescan of a removed file would.
	db.Delete(&model.Track{}, t2.ID)

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(q.Tracks))
	}
	if q.Tracks[0].ID != t1.ID || q.Tracks[1].ID != t3.ID {
		t.Fatalf("unexpected surviving tracks: %d, %d", q.Tracks[0].ID, q.Tracks[1].ID)
	}
	// The current track (t3) shifted from slot 2 to slot 1.
	if q.CurrentIndex != 1 {
		t.Fatalf("CurrentIndex = %d, want 1 after the earlier track was deleted", q.CurrentIndex)
	}
}

// When the current track itself is gone the position is meaningless — resuming
// mid-song into a different track would be wrong, so it resets to the start of
// whatever fell into that slot.
func TestGetPlayQueueResetsPositionWhenCurrentTrackDeleted(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	if err := s.SavePlayQueue("admin", []uint{t1.ID, t2.ID}, 0, 90000, "c", time.Now()); err != nil {
		t.Fatal(err)
	}

	db.Delete(&model.Track{}, t1.ID)

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tracks) != 1 || q.Tracks[0].ID != t2.ID {
		t.Fatalf("unexpected queue after deleting the current track: %+v", q.Tracks)
	}
	if q.CurrentIndex != 0 {
		t.Fatalf("CurrentIndex = %d, want 0", q.CurrentIndex)
	}
	if q.PositionMs != 0 {
		t.Fatalf("PositionMs = %d, want 0 — the saved offset belonged to a track that no longer exists", q.PositionMs)
	}
}

// The queue's tracks are returned fully hydrated, because the handler renders
// them as Subsonic Child objects (album, artists, genres all appear there).
func TestGetPlayQueuePreloadsTrackRelations(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	genre := model.Genre{Name: "Jazz"}
	db.Create(&genre)
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3", Title: "One"}
	db.Create(&tr)
	db.Create(&model.TrackArtist{TrackID: tr.ID, ArtistID: artist.ID})
	db.Create(&model.TrackGenre{TrackID: tr.ID, GenreID: genre.ID})

	if err := s.SavePlayQueue("admin", []uint{tr.ID}, 0, 0, "c", time.Now()); err != nil {
		t.Fatal(err)
	}
	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	got := q.Tracks[0]
	if got.Album == nil || got.Album.Name != "Alb" {
		t.Fatal("expected Album preloaded on the queue's tracks")
	}
	if len(got.Artists) != 1 || got.Artists[0].Name != "A" {
		t.Fatal("expected Artists preloaded on the queue's tracks")
	}
	if len(got.Genres) != 1 || got.Genres[0].Name != "Jazz" {
		t.Fatal("expected Genres preloaded on the queue's tracks")
	}
}

// Queues belong to their owner; one user's save must not be readable as another's.
func TestPlayQueueIsScopedToOwner(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&tr)

	if err := s.SavePlayQueue("alice", []uint{tr.ID}, 0, 0, "c", time.Now()); err != nil {
		t.Fatal(err)
	}
	q, err := s.GetPlayQueue("bob")
	if err != nil {
		t.Fatal(err)
	}
	if q != nil {
		t.Fatal("expected bob to have no queue of his own")
	}
}

// After orphan cleanup physically REMOVES entry rows, the surviving rows keep
// their original sort_order — so slot numbers have gaps. CurrentIndex must be
// matched against the stored sort_order, not the position of a row in the loaded
// slice, or the current track is misidentified and its position is thrown away.
func TestGetPlayQueueAfterOrphanCleanupKeepsCurrentPosition(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "2.mp3", FilePath: "/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	// Playing the SECOND slot, 90s in.
	if err := s.SavePlayQueue("admin", []uint{t1.ID, t2.ID}, 1, 90000, "c", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Delete the earlier track and run the cleanup a scan would, which drops its
	// entry row and leaves the survivor at sort_order 1.
	db.Delete(&model.Track{}, t1.ID)
	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Tracks) != 1 || q.Tracks[0].ID != t2.ID {
		t.Fatalf("unexpected surviving queue: %+v", q.Tracks)
	}
	if q.CurrentIndex != 0 {
		t.Fatalf("CurrentIndex = %d, want 0", q.CurrentIndex)
	}
	if q.PositionMs != 90000 {
		t.Fatalf("PositionMs = %d, want 90000 — the current track survived, so its offset must too", q.PositionMs)
	}
}

// When the current track is gone, the replacement must be the track that took
// over its place in the SURVIVING order — counted by how many earlier slots
// survived, not by reusing the stale pre-cleanup slot number.
func TestGetPlayQueueCurrentFallsToTheSlotThatTookOver(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tracks := make([]model.Track, 0, 4)
	for i := 1; i <= 4; i++ {
		tr := model.Track{AlbumID: album.ID, Filename: string(rune('0'+i)) + ".mp3", FilePath: "/" + string(rune('0'+i)) + ".mp3"}
		db.Create(&tr)
		tracks = append(tracks, tr)
	}
	ids := []uint{tracks[0].ID, tracks[1].ID, tracks[2].ID, tracks[3].ID}
	// Playing slot 1 (the second track).
	if err := s.SavePlayQueue("admin", ids, 1, 45000, "c", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Remove everything before the current track AND the current track itself,
	// then clean up. Every surviving slot now sits AFTER the old current index, so
	// reusing that stale number would skip the track that actually took over.
	db.Delete(&model.Track{}, tracks[0].ID)
	db.Delete(&model.Track{}, tracks[1].ID)
	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}

	q, err := s.GetPlayQueue("admin")
	if err != nil {
		t.Fatal(err)
	}
	// Survivors are the 3rd and 4th tracks, now at positions 0 and 1.
	if len(q.Tracks) != 2 || q.Tracks[0].ID != tracks[2].ID || q.Tracks[1].ID != tracks[3].ID {
		t.Fatalf("unexpected survivors: %+v", q.Tracks)
	}
	// No slot before the old current survived, so the track that took its place is
	// the first survivor. Clamping the stale index 1 would wrongly pick the second.
	if q.CurrentIndex != 0 {
		t.Fatalf("CurrentIndex = %d, want 0 (the track that took over the current slot)", q.CurrentIndex)
	}
	if q.PositionMs != 0 {
		t.Fatalf("PositionMs = %d, want 0 — the current track is gone", q.PositionMs)
	}
}

// Orphan cleanup after a scan must not leave entries pointing at deleted tracks.
func TestOrphanedPlayQueueEntriesAreRemoved(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tr := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&tr)
	if err := s.SavePlayQueue("admin", []uint{tr.ID}, 0, 0, "c", time.Now()); err != nil {
		t.Fatal(err)
	}
	db.Delete(&model.Track{}, tr.ID)

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.PlayQueueEntry{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected orphaned play queue entries removed, %d remain", n)
	}
}
