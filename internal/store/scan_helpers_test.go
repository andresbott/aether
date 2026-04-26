package store_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
)

func TestBulkUpdateLastSeen(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3"}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/music/02.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	now := time.Now()
	if err := s.BulkUpdateLastSeen([]string{"/music/01.mp3", "/music/02.mp3"}, now); err != nil {
		t.Fatal(err)
	}
	var track model.Track
	db.First(&track, t1.ID)
	if track.LastSeenAt.Before(now.Add(-time.Second)) {
		t.Fatal("expected LastSeenAt to be updated")
	}
}

func TestFilterChanged(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	track := model.Track{
		AlbumID: album.ID, Filename: "01.mp3", FilePath: "/music/01.mp3",
		FileModTime: old,
	}
	db.Create(&track)
	modMap, err := s.FilterChanged([]string{"/music/01.mp3", "/music/new.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := modMap["/music/01.mp3"]; !ok {
		t.Fatal("expected existing track in map")
	}
	if _, ok := modMap["/music/new.mp3"]; ok {
		t.Fatal("new file should not be in map")
	}
}

func TestCleanup(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "a"}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	now := time.Now()
	old := now.Add(-time.Hour)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", LastSeenAt: now}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", LastSeenAt: old}
	db.Create(&t1)
	db.Create(&t2)
	db.Model(&t1).Association("Artists").Replace([]*model.Artist{&artist})
	db.Model(&t2).Association("Artists").Replace([]*model.Artist{&artist})
	if err := s.Cleanup(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 track after cleanup, got %d", count)
	}
}

func TestDeleteOrphanedAggregates(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "Orphan", NameNorm: "orphan"}
	db.Create(&artist)
	album := model.Album{Name: "Orphan", NameNorm: "orphan", AlbumArtistNorm: "orphan"}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	// Album has no tracks, artist has no track-membership — both should be cleaned.

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}

	var albumCount, artistCount int64
	db.Model(&model.Album{}).Count(&albumCount)
	db.Model(&model.Artist{}).Count(&artistCount)
	if albumCount != 0 {
		t.Fatalf("expected 0 albums, got %d", albumCount)
	}
	if artistCount != 0 {
		t.Fatalf("expected 0 artists, got %d", artistCount)
	}
}

func TestDeleteTracksNotSeenSince(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	now := time.Now()
	old := now.Add(-time.Hour)
	t1 := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3", LastSeenAt: now}
	t2 := model.Track{AlbumID: album.ID, Filename: "02.mp3", FilePath: "/02.mp3", LastSeenAt: old}
	db.Create(&t1)
	db.Create(&t2)

	if err := s.DeleteTracksNotSeenSince(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&model.Track{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 track, got %d", count)
	}
}

func TestDeleteOrphanedAggregatesCleansReferences(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	playlist := model.Playlist{Name: "P"}
	db.Create(&playlist)
	db.Create(&model.PlaylistTrack{PlaylistID: playlist.ID, TrackID: track.ID, SortOrder: 0})
	db.Create(&model.StarredItem{ItemType: "track", ItemID: track.ID})
	db.Create(&model.PlayHistory{TrackID: track.ID, PlayedAt: time.Now()})

	// Delete the track directly (no cleanup yet) — references now dangle.
	if err := db.Delete(&track).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}

	var ptCount, siCount, phCount int64
	db.Model(&model.PlaylistTrack{}).Count(&ptCount)
	db.Model(&model.StarredItem{}).Count(&siCount)
	db.Model(&model.PlayHistory{}).Count(&phCount)
	if ptCount != 0 || siCount != 0 || phCount != 0 {
		t.Fatalf("expected all references cleaned, got pt=%d si=%d ph=%d", ptCount, siCount, phCount)
	}
}

func TestDeleteOrphanedAggregatesPreservesLiveStarredItems(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "A", NameNorm: "a"}
	db.Create(&artist)
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	track := model.Track{AlbumID: album.ID, Filename: "01.mp3", FilePath: "/01.mp3"}
	db.Create(&track)
	db.Model(&track).Association("Artists").Replace([]*model.Artist{&artist})
	db.Create(&model.StarredItem{ItemType: "track", ItemID: track.ID})
	db.Create(&model.StarredItem{ItemType: "album", ItemID: album.ID})
	db.Create(&model.StarredItem{ItemType: "artist", ItemID: artist.ID})

	if err := s.DeleteOrphanedAggregates(); err != nil {
		t.Fatal(err)
	}

	var siCount int64
	db.Model(&model.StarredItem{}).Count(&siCount)
	if siCount != 3 {
		t.Fatalf("expected all 3 starred items preserved (none orphaned), got %d", siCount)
	}
}
