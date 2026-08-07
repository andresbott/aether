package store_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// seedAlbum creates an album with optional genres and returns it.
func seedAlbum(t *testing.T, s *store.Store, name string, genres ...*model.Genre) model.Album {
	t.Helper()
	al := model.Album{Name: name, NameNorm: name, AlbumArtistNorm: "x", Genres: genres}
	if err := s.DB().Create(&al).Error; err != nil {
		t.Fatal(err)
	}
	return al
}

// seedTrack attaches a track to an album, optionally in a library and with genres.
func seedTrack(t *testing.T, s *store.Store, al model.Album, libraryID uint, genres ...*model.Genre) model.Track {
	t.Helper()
	// FilePath must be unique - use album ID to ensure it
	path := fmt.Sprintf("/track_%d_%d.mp3", al.ID, libraryID)
	tr := model.Track{
		Title:    al.Name + " track",
		AlbumID:  al.ID,
		FilePath: path,
		Filename: path,
		Genres:   genres,
	}
	if libraryID != 0 {
		tr.LibraryID = libraryID
	}
	if err := s.DB().Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	return tr
}

func seedGenre(t *testing.T, s *store.Store, name string) *model.Genre {
	t.Helper()
	g := model.Genre{Name: name}
	if err := s.DB().Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	return &g
}

func TestDiscoveryFeedReturnsAlbumsAndPlaylists(t *testing.T) {
	s := testStore(t)
	seedAlbum(t, s, "Album A")
	pl := model.Playlist{Name: "PL", Owner: "admin"}
	if err := s.DB().Create(&pl).Error; err != nil {
		t.Fatal(err)
	}

	items, err := s.DiscoveryFeed("admin", 10, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var albums, playlists int
	for _, it := range items {
		switch it.Kind {
		case "al":
			albums++
		case "pl":
			playlists++
		default:
			t.Fatalf("unexpected kind %q", it.Kind)
		}
	}
	if albums != 1 || playlists != 1 {
		t.Fatalf("got %d albums and %d playlists, want 1 and 1", albums, playlists)
	}
}

func TestDiscoveryFeedRanksAreSequentialFromOffset(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 6; i++ {
		seedAlbum(t, s, string(rune('A'+i)))
	}
	items, err := s.DiscoveryFeed("admin", 3, 2, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	for i, it := range items {
		if it.Rank != 2+i {
			t.Fatalf("item %d has Rank %d, want %d", i, it.Rank, 2+i)
		}
	}
}

func TestDiscoveryFeedPagesWithoutGapOrOverlap(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 10; i++ {
		seedAlbum(t, s, string(rune('A'+i)))
	}
	page1, err := s.DiscoveryFeed("admin", 5, 0, 42, nil)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := s.DiscoveryFeed("admin", 5, 5, 42, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, it := range append(page1, page2...) {
		key := fmt.Sprintf("%s-%d-%d", it.Kind, it.AlbumID, it.PlaylistID)
		if seen[key] {
			t.Fatalf("item %s appeared on both pages", key)
		}
		seen[key] = true
	}
	if len(seen) != 10 {
		t.Fatalf("two pages covered %d distinct items, want 10", len(seen))
	}
}

func TestGetAlbumsByIDsLoadsRequestedAlbumsOnly(t *testing.T) {
	s := testStore(t)
	a := seedAlbum(t, s, "A")
	b := seedAlbum(t, s, "B")
	seedAlbum(t, s, "C")

	albums, err := s.GetAlbumsByIDs([]uint{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 2 {
		t.Fatalf("got %d albums, want 2", len(albums))
	}
	got := map[uint]bool{}
	for i := range albums {
		got[albums[i].ID] = true
	}
	if !got[a.ID] || !got[b.ID] {
		t.Fatalf("wrong albums returned: %v", got)
	}
}

func TestGetAlbumsByIDsOnEmptyInput(t *testing.T) {
	s := testStore(t)
	albums, err := s.GetAlbumsByIDs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 0 {
		t.Fatalf("got %d albums for no ids, want 0", len(albums))
	}
}

// The paging guarantee, tested the way the feed is actually used: two SEPARATE
// requests at different offsets against one library. The ranks must form a
// contiguous run with no id served twice.
//
// This is the regression test for a real defect. An earlier draft sized the pool
// as (offset+size)*headroom, so page 2 scored candidates page 1 never saw; a
// high-scoring newcomer then landed inside ranks page 1 had already served and
// shifted everything down. Ranking is a sort — adding candidates moves the ones
// already there, whatever the pool contains.
func TestDiscoveryFeedRanksStayStableAcrossOffsets(t *testing.T) {
	s := testStore(t)
	// A spread of ages and play counts so scores genuinely differ, plus one
	// stand-out that would jump to rank 0 if it entered the pool late.
	for i := 0; i < 40; i++ {
		al := seedAlbum(t, s, string(rune('A'+i%26))+string(rune('a'+i/26)))
		tr := seedTrack(t, s, al, 0)
		if i%3 == 0 {
			if err := s.RecordPlay("admin", tr.ID, time.Now().Add(-time.Duration(i)*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}
	standout := seedAlbum(t, s, "Standout")
	stTrack := seedTrack(t, s, standout, 0)
	for i := 0; i < 50; i++ {
		if err := s.RecordPlay("admin", stTrack.ID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Star("admin", "album", standout.ID); err != nil {
		t.Fatal(err)
	}

	const seed = int64(4242)
	var all []store.DiscoveryItem
	for offset := 0; offset < 40; offset += 10 {
		page, err := s.DiscoveryFeed("admin", 10, offset, seed, nil)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, page...)
	}

	seenID := map[string]int{}
	for i, it := range all {
		if it.Rank != i {
			t.Fatalf("item %d has Rank %d, want %d — ranks are not contiguous across pages", i, it.Rank, i)
		}
		key := fmt.Sprintf("%s-%d-%d", it.Kind, it.AlbumID, it.PlaylistID)
		if prev, dup := seenID[key]; dup {
			t.Fatalf("%s served at rank %d and again at rank %d — the pool is not stable across offsets",
				key, prev, it.Rank)
		}
		seenID[key] = it.Rank
	}
}

// The pool size must not depend on offset. Requesting one big page and stitching
// small pages together must produce the identical sequence — if the pool grew with
// offset, the stitched version would diverge.
func TestDiscoveryFeedPagedMatchesSingleShot(t *testing.T) {
	s := testStore(t)
	// Create 60 albums: enough that (offset+size)*K pools would genuinely differ.
	for i := 0; i < 60; i++ {
		al := seedAlbum(t, s, fmt.Sprintf("Album%02d", i))
		tr := seedTrack(t, s, al, 0)
		if i%2 == 0 {
			if err := s.RecordPlay("admin", tr.ID, time.Now().Add(-time.Duration(i)*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}
	const seed = int64(99)
	single, err := s.DiscoveryFeed("admin", 48, 0, seed, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stitched []store.DiscoveryItem
	for offset := 0; offset < 48; offset += 12 {
		page, err := s.DiscoveryFeed("admin", 12, offset, seed, nil)
		if err != nil {
			t.Fatal(err)
		}
		stitched = append(stitched, page...)
	}
	if len(stitched) != len(single) {
		t.Fatalf("stitched %d items, single-shot %d", len(stitched), len(single))
	}
	for i := range single {
		if stitched[i] != single[i] {
			t.Fatalf("rank %d: stitched %+v != single-shot %+v", i, stitched[i], single[i])
		}
	}
}

// No ORDER BY RANDOM() anywhere in candidate gathering: repeated identical
// requests must return byte-identical results.
//
// A small fixture is deliberate. Mutating the never-played query to ORDER BY
// RANDOM() does NOT change the output at discoveryPoolSize = 2000, at any fixture
// size — `newest` alone gathers 2000 candidates, so every never-played album that
// could win the ranking is already in the pool and the sampled ordering is
// redundant. Verified up to 6000 albums. What this test actually guards is the
// cheap, valuable property: that two identical requests agree.
func TestDiscoveryFeedCandidateGatheringIsDeterministic(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 30; i++ {
		al := seedAlbum(t, s, fmt.Sprintf("Album%02d", i))
		tr := seedTrack(t, s, al, 0)
		if i%5 == 0 {
			if err := s.RecordPlay("admin", tr.ID, time.Now().Add(-time.Duration(i)*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
	}
	first, err := s.DiscoveryFeed("admin", 10, 0, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		again, err := s.DiscoveryFeed("admin", 10, 0, 7, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(again) != len(first) {
			t.Fatalf("attempt %d returned %d items, first call returned %d", attempt, len(again), len(first))
		}
		for i := range first {
			if first[i] != again[i] {
				t.Fatalf("attempt %d rank %d differed: %+v vs %+v — candidate gathering is not deterministic",
					attempt, i, first[i], again[i])
			}
		}
	}
}

// Never-played albums must reach the pool even when every play-driven ordering is
// saturated, or the rediscovery quota has nothing to draw from.
func TestDiscoveryFeedIncludesNeverPlayedAlbums(t *testing.T) {
	s := testStore(t)
	// Enough played albums to fill the play-driven orderings.
	for i := 0; i < 5; i++ {
		al := seedAlbum(t, s, "Played"+string(rune('A'+i)))
		tr := seedTrack(t, s, al, 0)
		if err := s.RecordPlay("admin", tr.ID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	forgotten := seedAlbum(t, s, "Forgotten")
	seedTrack(t, s, forgotten, 0)

	items, err := s.DiscoveryFeed("admin", 50, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Kind == "al" && it.AlbumID == forgotten.ID {
			return
		}
	}
	t.Fatal("never-played album absent from the feed; the rediscovery pool would be empty")
}

func TestDiscoveryFeedIsStableForOneSeed(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 8; i++ {
		seedAlbum(t, s, string(rune('A'+i)))
	}
	a, err := s.DiscoveryFeed("admin", 8, 0, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.DiscoveryFeed("admin", 8, 0, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("rank %d differed between identical calls: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestDiscoveryFeedRespectsLibraryFilter(t *testing.T) {
	s := testStore(t)
	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	if err := s.DB().Create(&lib1).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Create(&lib2).Error; err != nil {
		t.Fatal(err)
	}
	inLib1 := seedAlbum(t, s, "In L1")
	inLib2 := seedAlbum(t, s, "In L2")
	seedTrack(t, s, inLib1, lib1.ID)
	seedTrack(t, s, inLib2, lib2.ID)

	items, err := s.DiscoveryFeed("admin", 10, 0, 1, &store.DiscoveryFilter{LibraryID: &lib1.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Kind == "al" && it.AlbumID == inLib2.ID {
			t.Fatal("album from the other library leaked through the filter")
		}
	}
}

// The horizon is what bounds the taste query. A play past it must not shape the
// profile, so an album in that stale genre must not be boosted by it.
func TestTasteProfileIgnoresPlaysPastTheHorizon(t *testing.T) {
	s := testStore(t)
	stale := seedGenre(t, s, "Stale")
	fresh := seedGenre(t, s, "Fresh")

	staleAlbum := seedAlbum(t, s, "Stale Album", stale)
	freshAlbum := seedAlbum(t, s, "Fresh Album", fresh)
	staleTrack := seedTrack(t, s, staleAlbum, 0, stale)
	freshTrack := seedTrack(t, s, freshAlbum, 0, fresh)

	// One play far past the horizon in the stale genre, one recent in the fresh.
	if err := s.RecordPlay("admin", staleTrack.ID, time.Now().Add(-discoveryHorizonPlus())); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlay("admin", freshTrack.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	// A third album in the fresh genre, never played, should outrank a
	// never-played album in the stale genre because only Fresh is in the profile.
	freshUnplayed := seedAlbum(t, s, "Fresh Unplayed", fresh)
	staleUnplayed := seedAlbum(t, s, "Stale Unplayed", stale)

	items, err := s.DiscoveryFeed("admin", 50, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	rankOf := map[uint]int{}
	for _, it := range items {
		if it.Kind == "al" {
			rankOf[it.AlbumID] = it.Rank
		}
	}
	fr, okF := rankOf[freshUnplayed.ID]
	sr, okS := rankOf[staleUnplayed.ID]
	if !okF || !okS {
		t.Fatalf("expected both unplayed albums in the feed, got %v", rankOf)
	}
	if fr >= sr {
		t.Fatalf("fresh-genre album ranked %d, stale-genre %d — the horizon was not applied", fr, sr)
	}
}

func TestDiscoveryFeedOnEmptyLibrary(t *testing.T) {
	s := testStore(t)
	items, err := s.DiscoveryFeed("admin", 10, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items from an empty library, want 0", len(items))
	}
}

func TestDiscoveryFeedWithNoPlayHistory(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 4; i++ {
		seedAlbum(t, s, string(rune('A'+i)))
	}
	items, err := s.DiscoveryFeed("admin", 10, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want all 4 albums even with no play history", len(items))
	}
}

func TestDiscoveryFeedStarredAlbumOutranksPlainOne(t *testing.T) {
	s := testStore(t)
	plain := seedAlbum(t, s, "Plain")
	starred := seedAlbum(t, s, "Starred")
	if err := s.Star("admin", "album", starred.ID); err != nil {
		t.Fatal(err)
	}
	items, err := s.DiscoveryFeed("admin", 10, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	rankOf := map[uint]int{}
	for _, it := range items {
		if it.Kind == "al" {
			rankOf[it.AlbumID] = it.Rank
		}
	}
	if rankOf[starred.ID] >= rankOf[plain.ID] {
		t.Fatalf("starred album ranked %d, plain %d — favorite weight not applied",
			rankOf[starred.ID], rankOf[plain.ID])
	}
}

// discoveryHorizonPlus is the age of a play just past the taste horizon.
func discoveryHorizonPlus() time.Duration {
	return 731 * 24 * time.Hour
}

// TestDiscoveryFeedPoolDoesNotGrowWithOffset ensures the pool is truly constant.
// This catches the (offset+size)*K bug directly by checking whether items at
// a high absolute rank are identical when fetched from offset 0 vs a deep offset.
// If the pool size depends on offset, the deep-offset call would score additional
// candidates and shift the rankings.
func TestDiscoveryFeedPoolDoesNotGrowWithOffset(t *testing.T) {
	s := testStore(t)
	// Create 80 albums with varied play patterns.
	for i := 0; i < 80; i++ {
		al := seedAlbum(t, s, fmt.Sprintf("Album%03d", i))
		tr := seedTrack(t, s, al, 0)
		if i%4 == 0 {
			if err := s.RecordPlay("admin", tr.ID, time.Now().Add(-time.Duration(i)*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}

	const seed = int64(7777)
	// Fetch a big page from offset 0 containing ranks 40-49.
	bigPage, err := s.DiscoveryFeed("admin", 50, 0, seed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bigPage) < 50 {
		t.Skipf("only %d items available, need 50 to test offset 40", len(bigPage))
	}

	// Fetch the same ranks from offset 40 with size 10.
	deepPage, err := s.DiscoveryFeed("admin", 10, 40, seed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(deepPage) != 10 {
		t.Fatalf("expected 10 items at offset 40, got %d", len(deepPage))
	}

	// If the pool size were (offset+size)*K, the deep-offset call would score
	// candidates the offset-0 call never saw, potentially pushing different items
	// into the 40-49 window. With a constant pool, ranks 40-49 are identical.
	for i := 0; i < 10; i++ {
		big := bigPage[40+i]
		deep := deepPage[i]
		if big != deep {
			t.Fatalf("rank %d: offset-0 fetched %+v, offset-40 fetched %+v — pool size is NOT constant (the (offset+size)*K bug)",
				40+i, big, deep)
		}
	}
}
