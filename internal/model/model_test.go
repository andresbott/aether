package model_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestArtistCRUD(t *testing.T) {
	db := testDB(t)

	artist := model.Artist{Name: "Björk", NameNorm: "bjork"}
	if err := db.Create(&artist).Error; err != nil {
		t.Fatal(err)
	}
	if artist.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	var found model.Artist
	if err := db.First(&found, artist.ID).Error; err != nil {
		t.Fatal(err)
	}
	if found.Name != "Björk" || found.NameNorm != "bjork" {
		t.Fatalf("unexpected: %+v", found)
	}
}

func TestArtistUniqueNameNorm(t *testing.T) {
	db := testDB(t)

	db.Create(&model.Artist{Name: "Björk", NameNorm: "bjork"})
	err := db.Create(&model.Artist{Name: "BJÖRK", NameNorm: "bjork"}).Error
	if err == nil {
		t.Fatal("expected unique constraint violation")
	}
}

func TestAlbumIdentityUnique(t *testing.T) {
	db := testDB(t)

	a1 := model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead", MBReleaseID: ""}
	if err := db.Create(&a1).Error; err != nil {
		t.Fatal(err)
	}
	a2 := model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead", MBReleaseID: ""}
	err := db.Create(&a2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for same album identity")
	}
}

func TestAlbumIdentityDifferentMBID(t *testing.T) {
	db := testDB(t)

	a1 := model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead", MBReleaseID: "release-1"}
	if err := db.Create(&a1).Error; err != nil {
		t.Fatal(err)
	}
	a2 := model.Album{Name: "OK Computer", NameNorm: "ok computer", AlbumArtistNorm: "radiohead", MBReleaseID: "release-2"}
	if err := db.Create(&a2).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTrackBelongsToAlbum(t *testing.T) {
	db := testDB(t)

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)

	track := model.Track{
		AlbumID:  album.ID,
		Filename: "01-everything.flac",
		FilePath: "/music/radiohead/kid-a/01-everything.flac",
		Title:    "Everything In Its Right Place",
		Duration: 250,
	}
	if err := db.Create(&track).Error; err != nil {
		t.Fatal(err)
	}

	var loaded model.Track
	db.Preload("Album").First(&loaded, track.ID)
	if loaded.Album == nil || loaded.Album.Name != "Kid A" {
		t.Fatal("expected album preload")
	}
}

func TestTrackArtistManyToMany(t *testing.T) {
	db := testDB(t)

	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&artist)

	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)

	track := model.Track{
		AlbumID:  album.ID,
		Filename: "01.flac",
		FilePath: "/music/01.flac",
	}
	db.Create(&track)

	_ = db.Model(&track).Association("Artists").Append(&artist)

	var loaded model.Track
	db.Preload("Artists").First(&loaded, track.ID)
	if len(loaded.Artists) != 1 || loaded.Artists[0].NameNorm != "radiohead" {
		t.Fatalf("expected 1 artist, got %d", len(loaded.Artists))
	}
}

func TestGenreUnique(t *testing.T) {
	db := testDB(t)

	db.Create(&model.Genre{Name: "Rock"})
	err := db.Create(&model.Genre{Name: "Rock"}).Error
	if err == nil {
		t.Fatal("expected unique constraint violation")
	}
}

func TestPlaylistCRUD(t *testing.T) {
	db := testDB(t)
	playlist := model.Playlist{Name: "Favourites", Owner: "admin", Public: true}
	if err := db.Create(&playlist).Error; err != nil {
		t.Fatal(err)
	}
	if playlist.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	var found model.Playlist
	if err := db.First(&found, playlist.ID).Error; err != nil {
		t.Fatal(err)
	}
	if found.Name != "Favourites" || found.Owner != "admin" {
		t.Fatalf("unexpected: %+v", found)
	}
}

func TestPlaylistTrackOrdering(t *testing.T) {
	db := testDB(t)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", Title: "First"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", Title: "Second"}
	db.Create(&t1)
	db.Create(&t2)
	pl := model.Playlist{Name: "Test"}
	db.Create(&pl)
	db.Create(&model.PlaylistTrack{PlaylistID: pl.ID, TrackID: t2.ID, SortOrder: 0})
	db.Create(&model.PlaylistTrack{PlaylistID: pl.ID, TrackID: t1.ID, SortOrder: 1})
	var pts []model.PlaylistTrack
	db.Where("playlist_id = ?", pl.ID).Order("sort_order").Find(&pts)
	if len(pts) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(pts))
	}
	if pts[0].TrackID != t2.ID || pts[1].TrackID != t1.ID {
		t.Fatal("unexpected ordering")
	}
}

func TestStarredItemUnique(t *testing.T) {
	db := testDB(t)
	s1 := model.StarredItem{ItemType: "album", ItemID: 1}
	if err := db.Create(&s1).Error; err != nil {
		t.Fatal(err)
	}
	s2 := model.StarredItem{ItemType: "album", ItemID: 1}
	err := db.Create(&s2).Error
	if err == nil {
		t.Fatal("expected unique constraint violation")
	}
	s3 := model.StarredItem{ItemType: "artist", ItemID: 1}
	if err := db.Create(&s3).Error; err != nil {
		t.Fatal("different types should not conflict:", err)
	}
}

func TestPlayHistoryCRUD(t *testing.T) {
	db := testDB(t)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	ph := model.PlayHistory{TrackID: track.ID, PlayedAt: time.Now()}
	if err := db.Create(&ph).Error; err != nil {
		t.Fatal(err)
	}
	if ph.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestArtistHasMBIDAndFetchState(t *testing.T) {
	a := model.Artist{MBArtistID: "mbid-x"}
	if a.MBArtistID != "mbid-x" {
		t.Fatal("MBArtistID field missing")
	}
	if a.LastImageFetchAt != nil {
		t.Fatal("LastImageFetchAt should default to nil")
	}
}
