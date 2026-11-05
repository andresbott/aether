package store_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

const (
	mbArtist = "11111111-1111-1111-1111-111111111111"
	mbGroup  = "22222222-2222-2222-2222-222222222222"
	mbRec    = "33333333-3333-3333-3333-333333333333"
	mbOther  = "44444444-4444-4444-4444-444444444444"
)

// seedWishlistCatalog writes one artist, one album and one track carrying the
// three test MBIDs, and returns their ids.
func seedWishlistCatalog(t *testing.T, s *store.Store) (artistID, albumID, trackID uint) {
	t.Helper()
	db := s.DB()
	ar := model.Artist{Name: "Nirvana", NameNorm: "nirvana", MBArtistID: mbArtist}
	if err := db.Create(&ar).Error; err != nil {
		t.Fatal(err)
	}
	al := model.Album{Name: "Nevermind", NameNorm: "nevermind", AlbumArtistNorm: "nirvana", MBReleaseGroupID: mbGroup}
	if err := db.Create(&al).Error; err != nil {
		t.Fatal(err)
	}
	tr := model.Track{AlbumID: al.ID, Filename: "a.flac", FilePath: "/m/a.flac", Title: "Lithium", MBRecordingID: mbRec}
	if err := db.Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	return ar.ID, al.ID, tr.ID
}

func TestAddWishlistItemsMatchesImmediately(t *testing.T) {
	s := testStore(t)
	artistID, albumID, trackID := seedWishlistCatalog(t, s)

	got, err := s.AddWishlistItems([]model.WishlistItem{
		{Kind: model.WishlistArtist, MBID: mbArtist, Title: "Nirvana"},
		{Kind: model.WishlistAlbum, MBID: mbGroup, Title: "Nevermind"},
		{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"},
		{Kind: model.WishlistAlbum, MBID: mbOther, Title: "In Utero"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rows, got %d", len(got))
	}
	if got[0].MatchID != artistID || got[0].MatchedAt == nil {
		t.Errorf("artist not matched: %+v", got[0])
	}
	if got[1].MatchID != albumID {
		t.Errorf("album not matched: %+v", got[1])
	}
	if got[2].MatchID != trackID || got[2].MatchAlbumID != albumID {
		t.Errorf("song not matched to track+album: %+v", got[2])
	}
	if got[3].MatchID != 0 || got[3].MatchedAt != nil {
		t.Errorf("unknown album must stay unmatched: %+v", got[3])
	}
}

func TestAddWishlistItemsIsIdempotent(t *testing.T) {
	s := testStore(t)
	first, err := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistAlbum, MBID: mbGroup, Title: "Nevermind", Note: "keep"}})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistAlbum, MBID: mbGroup, Title: "Changed"}})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != first[0].ID || again[0].Title != "Nevermind" || again[0].Note != "keep" {
		t.Fatalf("re-adding must return the stored row untouched, got %+v", again[0])
	}
	// The same MBID under another kind is a different entry.
	other, err := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbGroup, Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if other[0].ID == first[0].ID {
		t.Fatal("kind is part of the identity")
	}
	all, _ := s.ListWishlist()
	if len(all) != 2 {
		t.Fatalf("want 2 entries, got %d", len(all))
	}
}

// An entry added by hand has no MBID: two of them never collide, and neither is
// ever matched — not even when the catalog holds tracks with no MBID either.
func TestAddWishlistItemsManualEntries(t *testing.T) {
	s := testStore(t)
	tr := model.Track{AlbumID: 1, Filename: "a.flac", FilePath: "/m/a.flac"}
	if err := s.DB().Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	got, err := s.AddWishlistItems([]model.WishlistItem{
		{Kind: model.WishlistSong, Title: "Live bootleg", Note: "the 1993 radio session"},
		{Kind: model.WishlistSong, Title: "Live bootleg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID == got[1].ID {
		t.Fatal("two manual entries are two entries")
	}
	if got[0].Note != "the 1993 radio session" || !got[0].Manual() {
		t.Fatalf("unexpected %+v", got[0])
	}
	stats, err := s.ReconcileWishlist()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Matched != 0 {
		t.Fatalf("a manual entry must never match, got %+v", stats)
	}
}

func TestAddWishlistItemsRejectsUnknownKind(t *testing.T) {
	s := testStore(t)
	_, err := s.AddWishlistItems([]model.WishlistItem{{Kind: "playlist", MBID: mbGroup, Title: "x"}})
	if !errors.Is(err, store.ErrInvalidWishlistKind) {
		t.Fatalf("want ErrInvalidWishlistKind, got %v", err)
	}
}

func TestReconcileWishlistFindsAndLoses(t *testing.T) {
	s := testStore(t)
	items, err := s.AddWishlistItems([]model.WishlistItem{
		{Kind: model.WishlistAlbum, MBID: mbGroup, Title: "Nevermind"},
		{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].MatchID != 0 {
		t.Fatal("nothing is in the catalog yet")
	}

	_, albumID, trackID := seedWishlistCatalog(t, s)
	stats, err := s.ReconcileWishlist()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Found != 2 || stats.Matched != 2 || stats.Lost != 0 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	it, _ := s.GetWishlistItem(items[1].ID)
	if it.MatchID != trackID || it.MatchAlbumID != albumID || it.MatchedAt == nil {
		t.Fatalf("song not matched: %+v", it)
	}
	firstSeen := *it.MatchedAt

	// A second pass with nothing changed reports nothing new and keeps the time.
	stats, _ = s.ReconcileWishlist()
	if stats.Found != 0 || stats.Matched != 2 {
		t.Fatalf("idle pass changed something: %+v", stats)
	}
	it, _ = s.GetWishlistItem(items[1].ID)
	if !it.MatchedAt.Equal(firstSeen) {
		t.Fatal("matched_at moved on an idle pass")
	}

	// The album leaves the catalog: its entry and the song on it are cleared.
	if err := s.DB().Exec("DELETE FROM tracks").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Exec("DELETE FROM albums").Error; err != nil {
		t.Fatal(err)
	}
	stats, _ = s.ReconcileWishlist()
	if stats.Lost != 2 || stats.Matched != 0 {
		t.Fatalf("unexpected stats after removal %+v", stats)
	}
	it, _ = s.GetWishlistItem(items[0].ID)
	if it.MatchID != 0 || it.MatchedAt != nil {
		t.Fatalf("album still matched: %+v", it)
	}
}

func TestReconcileWishlistKeepsFirstSeenWhenMatchMoves(t *testing.T) {
	s := testStore(t)
	_, albumID, _ := seedWishlistCatalog(t, s)
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistAlbum, MBID: mbGroup, Title: "Nevermind"}})
	seen := *items[0].MatchedAt

	// A rescan re-created the album under a new id.
	if err := s.DB().Exec("UPDATE albums SET id = ? WHERE id = ?", albumID+100, albumID).Error; err != nil {
		t.Fatal(err)
	}
	stats, _ := s.ReconcileWishlist()
	if stats.Found != 0 || stats.Lost != 0 || stats.Matched != 1 {
		t.Fatalf("a moved match is neither found nor lost: %+v", stats)
	}
	it, _ := s.GetWishlistItem(items[0].ID)
	if it.MatchID != albumID+100 || !it.MatchedAt.Equal(seen) {
		t.Fatalf("want new id with the first-seen time kept, got %+v", it)
	}
}

func TestWishlistCatalogMatchesPicksOldestRow(t *testing.T) {
	s := testStore(t)
	_, albumID, _ := seedWishlistCatalog(t, s)
	dup := model.Album{Name: "Nevermind (Deluxe)", NameNorm: "nevermind deluxe", AlbumArtistNorm: "nirvana", MBReleaseGroupID: mbGroup}
	if err := s.DB().Create(&dup).Error; err != nil {
		t.Fatal(err)
	}
	m, err := s.WishlistCatalogMatches(model.WishlistAlbum, []string{mbGroup, mbOther, ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[mbGroup].ID != albumID {
		t.Fatalf("want the oldest album, got %+v", m)
	}
	if _, err := s.WishlistCatalogMatches("genre", []string{mbGroup}); !errors.Is(err, store.ErrInvalidWishlistKind) {
		t.Fatalf("want ErrInvalidWishlistKind, got %v", err)
	}
}

// Some taggers write MBIDs uppercase; the catalog keeps them as spelled.
func TestWishlistMatchIsCaseInsensitive(t *testing.T) {
	s := testStore(t)
	tr := model.Track{AlbumID: 1, Filename: "a.flac", FilePath: "/m/a.flac", MBRecordingID: strings.ToUpper(mbRec)}
	if err := s.DB().Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	items, err := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: strings.ToUpper(mbRec), Title: "Lithium"}})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].MBID != mbRec {
		t.Errorf("stored MBID must be lowercase, got %q", items[0].MBID)
	}
	if items[0].MatchID != tr.ID {
		t.Fatalf("an uppercase catalog MBID must still match: %+v", items[0])
	}
}

func TestWishlistIDsByMBID(t *testing.T) {
	s := testStore(t)
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistArtist, MBID: mbArtist, Title: "Nirvana"}})
	ids, err := s.WishlistIDsByMBID(model.WishlistArtist, []string{mbArtist, mbOther})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[mbArtist] != items[0].ID {
		t.Fatalf("unexpected %+v", ids)
	}
	ids, _ = s.WishlistIDsByMBID(model.WishlistAlbum, []string{mbArtist})
	if len(ids) != 0 {
		t.Fatalf("kind must narrow the lookup, got %+v", ids)
	}
}

func TestUpdateWishlistItemEditsEveryFieldOfAManualEntry(t *testing.T) {
	s := testStore(t)
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, Title: "Radio sesion", Note: "1993"}})
	id := items[0].ID

	title, kind, note := "Radio session", model.WishlistAlbum, "BBC, 1993"
	it, err := s.UpdateWishlistItem(id, store.WishlistUpdate{Title: &title, Kind: &kind, Note: &note})
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != title || it.Kind != kind || it.Note != note {
		t.Fatalf("not every field changed: %+v", it)
	}
	stored, _ := s.GetWishlistItem(id)
	if stored.Title != title || stored.Kind != kind || stored.Note != note {
		t.Fatalf("not persisted: %+v", stored)
	}

	bad := model.WishlistKind("playlist")
	if _, err := s.UpdateWishlistItem(id, store.WishlistUpdate{Kind: &bad}); !errors.Is(err, store.ErrInvalidWishlistKind) {
		t.Fatalf("want ErrInvalidWishlistKind, got %v", err)
	}
	// An empty update changes nothing and is not an error.
	if got, err := s.UpdateWishlistItem(id, store.WishlistUpdate{}); err != nil || got.Title != title {
		t.Fatalf("empty update: %+v %v", got, err)
	}
}

// A MusicBrainz pick's title and kind are its snapshot: only its note changes.
func TestUpdateWishlistItemRefusesSnapshotFieldsOfAPick(t *testing.T) {
	s := testStore(t)
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"}})
	title := "Renamed"
	if _, err := s.UpdateWishlistItem(items[0].ID, store.WishlistUpdate{Title: &title}); !errors.Is(err, store.ErrWishlistNotManual) {
		t.Fatalf("want ErrWishlistNotManual, got %v", err)
	}
	kind := model.WishlistAlbum
	if _, err := s.UpdateWishlistItem(items[0].ID, store.WishlistUpdate{Kind: &kind}); !errors.Is(err, store.ErrWishlistNotManual) {
		t.Fatalf("want ErrWishlistNotManual, got %v", err)
	}
	stored, _ := s.GetWishlistItem(items[0].ID)
	if stored.Title != "Lithium" || stored.Kind != model.WishlistSong {
		t.Fatalf("a refused update must change nothing: %+v", stored)
	}
}

func TestWishlistNoteAndDelete(t *testing.T) {
	s := testStore(t)
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"}})
	id := items[0].ID

	note := "the live version"
	it, err := s.UpdateWishlistItem(id, store.WishlistUpdate{Note: &note})
	if err != nil || it.Note != note {
		t.Fatalf("note not updated: %+v %v", it, err)
	}
	if _, err := s.UpdateWishlistItem(id+99, store.WishlistUpdate{Note: &note}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.DeleteWishlistItem(id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWishlistItem(id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound on second delete, got %v", err)
	}
	if _, err := s.GetWishlistItem(id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
