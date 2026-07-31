package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

func TestStarAndUnstar(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("artist", artist.ID); err != nil {
		t.Fatal(err)
	} // idempotent
	starred, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 {
		t.Fatalf("expected 1 starred artist, got %d", len(starred.Artists))
	}
	if err := s.Unstar("artist", artist.ID); err != nil {
		t.Fatal(err)
	}
	starred, _ = s.GetStarred(nil)
	if len(starred.Artists) != 0 {
		t.Fatal("expected 0 starred artists after unstar")
	}
}

func TestGetStarredAll(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	_ = s.Star("artist", artist.ID)
	_ = s.Star("album", album.ID)
	_ = s.Star("track", track.ID)
	starred, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Artists) != 1 || len(starred.Albums) != 1 || len(starred.Tracks) != 1 {
		t.Fatalf("unexpected: artists=%d albums=%d tracks=%d", len(starred.Artists), len(starred.Albums), len(starred.Tracks))
	}
}

func TestGetStarredByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	artist1 := model.Artist{Name: "A1", NameNorm: "a1"}
	artist2 := model.Artist{Name: "A2", NameNorm: "a2"}
	db.Create(&artist1)
	db.Create(&artist2)

	album1 := model.Album{Name: "Alb1", NameNorm: "alb1", AlbumArtistNorm: "a1"}
	album2 := model.Album{Name: "Alb2", NameNorm: "alb2", AlbumArtistNorm: "a2"}
	db.Create(&album1)
	db.Create(&album2)

	t1 := model.Track{AlbumID: album1.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album2.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	_ = db.Model(&t1).Association("Artists").Replace([]*model.Artist{&artist1})
	_ = db.Model(&t2).Association("Artists").Replace([]*model.Artist{&artist2})

	if err := s.Star("artist", artist1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("artist", artist2.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("album", album1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("album", album2.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("track", t1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Star("track", t2.ID); err != nil {
		t.Fatal(err)
	}

	id1 := lib1.ID
	got, err := s.GetStarred(&store.StarredFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artists) != 1 || got.Artists[0].Name != "A1" {
		t.Fatalf("expected [A1], got %+v", got.Artists)
	}
	if len(got.Albums) != 1 || got.Albums[0].Name != "Alb1" {
		t.Fatalf("expected [Alb1], got %+v", got.Albums)
	}
	if len(got.Tracks) != 1 || got.Tracks[0].LibraryID != lib1.ID {
		t.Fatalf("expected 1 track in library 1, got %+v", got.Tracks)
	}

	all, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Artists) != 2 || len(all.Albums) != 2 || len(all.Tracks) != 2 {
		t.Fatalf("expected all starred items with nil filter, got %+v", all)
	}
}

func TestGetStarredIncludesPlaylistsNewestFirst(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	older := model.Playlist{Name: "Older"}
	newer := model.Playlist{Name: "Newer"}
	unstarred := model.Playlist{Name: "Plain"}
	db.Create(&older)
	db.Create(&newer)
	db.Create(&unstarred)

	// Explicit CreatedAt values: two stars created in the same test tick would
	// otherwise tie and make the order arbitrary.
	base := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	db.Create(&model.StarredItem{ItemType: "playlist", ItemID: older.ID, CreatedAt: base})
	db.Create(&model.StarredItem{ItemType: "playlist", ItemID: newer.ID, CreatedAt: base.Add(time.Hour)})

	starred, err := s.GetStarred(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred.Playlists) != 2 {
		t.Fatalf("expected 2 starred playlists, got %d", len(starred.Playlists))
	}
	if starred.Playlists[0].Name != "Newer" || starred.Playlists[1].Name != "Older" {
		t.Fatalf("expected [Newer Older], got [%s %s]", starred.Playlists[0].Name, starred.Playlists[1].Name)
	}
}

func TestStarredAt(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	starredPl := model.Playlist{Name: "Fav"}
	plainPl := model.Playlist{Name: "Plain"}
	db.Create(&starredPl)
	db.Create(&plainPl)

	at := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	db.Create(&model.StarredItem{ItemType: "playlist", ItemID: starredPl.ID, CreatedAt: at})

	got, err := s.StarredAt("playlist", []uint{starredPl.ID, plainPl.ID})
	if err != nil {
		t.Fatal(err)
	}
	if ts, ok := got[starredPl.ID]; !ok || !ts.Equal(at) {
		t.Fatalf("starred playlist timestamp = %v (ok=%v), want %v", ts, ok, at)
	}
	if _, ok := got[plainPl.ID]; ok {
		t.Fatal("unstarred playlist must be absent from the map")
	}
}

// Ids collide across item types (album 1 and track 1 both exist), so the lookup
// must filter on item_type or an album's star would leak onto a song.
func TestStarredAtIsScopedToItemType(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "1.mp3", FilePath: "/1.mp3"}
	db.Create(&track)

	if err := s.Star("album", album.ID); err != nil {
		t.Fatal(err)
	}

	albums, err := s.StarredAt("album", []uint{album.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := albums[album.ID]; !ok {
		t.Fatal("starred album must be present in the album lookup")
	}

	tracks, err := s.StarredAt("track", []uint{track.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tracks[track.ID]; ok {
		t.Fatal("an album star must not surface in the track lookup")
	}
}

func TestStarredAtWithNoIDs(t *testing.T) {
	s := testStore(t)
	got, err := s.StarredAt("album", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestOrphanedPlaylistStarsAreRemoved(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl := model.Playlist{Name: "Gone"}
	db.Create(&pl)
	if err := s.Star("playlist", pl.ID); err != nil {
		t.Fatal(err)
	}
	db.Delete(&model.Playlist{}, pl.ID)

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.StarredItem{}).Where("item_type = 'playlist'").Count(&n)
	if n != 0 {
		t.Fatalf("expected orphaned playlist stars removed, %d remain", n)
	}
}
