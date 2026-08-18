package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/unidecode"
	"gorm.io/gorm"
)

func TestFindOrCreateArtists(t *testing.T) {
	s := testStore(t)
	var artists []*model.Artist
	err := s.Transaction(func(tx *store.Store) error {
		var txErr error
		artists, txErr = tx.FindOrCreateArtists([]string{"Björk", "Radiohead"}, nil)
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(artists))
	}
	// Call again — should find existing
	var again []*model.Artist
	err = s.Transaction(func(tx *store.Store) error {
		var txErr error
		again, txErr = tx.FindOrCreateArtists([]string{"Björk"}, nil)
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != artists[0].ID {
		t.Fatal("expected same artist ID on second call")
	}
}

func TestFindOrCreateArtistsSetsMBID(t *testing.T) {
	s := testStore(t)
	var got []*model.Artist
	err := s.Transaction(func(tx *store.Store) error {
		var e error
		got, e = tx.FindOrCreateArtists([]string{"Björk"}, []string{"mbid-bjork"})
		return e
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got[0].MBArtistID != "mbid-bjork" {
		t.Fatalf("MBID not set, got %q", got[0].MBArtistID)
	}
	// Backfill: same artist, now with an MBID where it was set; calling again
	// with empty must not clear it.
	err = s.Transaction(func(tx *store.Store) error {
		var e error
		got, e = tx.FindOrCreateArtists([]string{"Björk"}, nil)
		return e
	})
	if err != nil || got[0].MBArtistID != "mbid-bjork" {
		t.Fatalf("MBID lost on re-find: %q (err %v)", got[0].MBArtistID, err)
	}
}

func TestFindOrCreateArtistsBackfillsMBID(t *testing.T) {
	s := testStore(t)
	// First call: create artist with no MBID.
	var first []*model.Artist
	err := s.Transaction(func(tx *store.Store) error {
		var e error
		first, e = tx.FindOrCreateArtists([]string{"Portishead"}, nil)
		return e
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if first[0].MBArtistID != "" {
		t.Fatalf("expected empty MBID on first create, got %q", first[0].MBArtistID)
	}
	// Second call: same artist but now with a real MBID — must backfill.
	var second []*model.Artist
	err = s.Transaction(func(tx *store.Store) error {
		var e error
		second, e = tx.FindOrCreateArtists([]string{"Portishead"}, []string{"mbid-portishead"})
		return e
	})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if second[0].ID != first[0].ID {
		t.Fatal("expected same artist row")
	}
	if second[0].MBArtistID != "mbid-portishead" {
		t.Fatalf("MBID not backfilled, got %q", second[0].MBArtistID)
	}
}

// errQueryBoom is the injected failure used to simulate a real DB error (not a
// missing row) on the lookup half of a find-or-create.
var errQueryBoom = errors.New("boom: simulated db failure")

// failQueries makes every SELECT on s fail with errQueryBoom while leaving
// INSERTs working, which is exactly the shape a find-or-create must not
// mistake for "row does not exist".
func failQueries(t *testing.T, s *store.Store) {
	t.Helper()
	err := s.DB().Callback().Query().Before("gorm:query").
		Register("test:fail_queries", func(tx *gorm.DB) {
			_ = tx.AddError(errQueryBoom)
		})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}
}

func TestFindOrCreateArtistsPropagatesQueryError(t *testing.T) {
	s := testStore(t)
	failQueries(t, s)

	_, err := s.FindOrCreateArtists([]string{"Björk"}, nil)
	if err == nil {
		t.Fatal("expected the DB error to propagate, got nil (a real failure was treated as not-found)")
	}
	if !errors.Is(err, errQueryBoom) {
		t.Fatalf("expected the injected DB error, got %v", err)
	}
}

func TestGetArtists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	a1 := model.Artist{Name: "Björk", NameNorm: "bjork"}
	a2 := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&a1)
	db.Create(&a2)
	alb1 := model.Album{Name: "Debut", NameNorm: "debut", AlbumArtistNorm: "bjork"}
	alb2 := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&alb1)
	db.Create(&alb2)
	_ = db.Model(&alb1).Association("Artists").Replace([]*model.Artist{&a1})
	_ = db.Model(&alb2).Association("Artists").Replace([]*model.Artist{&a2})
	artists, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(artists))
	}
}

func TestGetArtist(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	db.Create(&artist)
	album := model.Album{Name: "Kid A", NameNorm: "kid a", AlbumArtistNorm: "radiohead"}
	db.Create(&album)
	_ = db.Model(&album).Association("Artists").Replace([]*model.Artist{&artist})
	found, albums, err := s.GetArtist(artist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "Radiohead" {
		t.Fatalf("unexpected name: %s", found.Name)
	}
	if len(albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(albums))
	}
}

func TestGetArtistNotFound(t *testing.T) {
	s := testStore(t)
	_, _, err := s.GetArtist(9999)
	if err == nil {
		t.Fatal("expected error for missing artist")
	}
}

func TestSearchArtists(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	db.Create(&model.Artist{Name: "Radiohead", NameNorm: "radiohead"})
	db.Create(&model.Artist{Name: "Björk", NameNorm: "bjork"})
	results, err := s.SearchArtists("radio", 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Radiohead" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestGetArtistsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	seedArtistTrack(t, s, lib1.ID, "Alpha", "/l1/1.mp3")
	seedArtistTrack(t, s, lib2.ID, "Beta", "/l2/2.mp3")
	seedArtistTrack(t, s, lib1.ID, "Gamma", "/l1/3.mp3")

	id1 := lib1.ID
	got, err := s.GetArtists(&store.ArtistsFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected Alpha + Gamma, got %d: %+v", len(got), got)
	}

	all, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 artists with nil filter, got %d", len(all))
	}
}

func TestGetArtistAlbumCountsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	artist := model.Artist{Name: "Alpha", NameNorm: "alpha"}
	db.Create(&artist)

	alb1 := model.Album{Name: "A1", NameNorm: "a1", AlbumArtistNorm: "alpha"}
	alb2 := model.Album{Name: "A2", NameNorm: "a2", AlbumArtistNorm: "alpha"}
	db.Create(&alb1)
	db.Create(&alb2)
	_ = db.Model(&alb1).Association("Artists").Replace([]*model.Artist{&artist})
	_ = db.Model(&alb2).Association("Artists").Replace([]*model.Artist{&artist})

	db.Create(&model.Track{AlbumID: alb1.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"})
	db.Create(&model.Track{AlbumID: alb2.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"})

	id1 := lib1.ID
	counts, err := s.GetArtistAlbumCounts(&store.ArtistsFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if counts[artist.ID] != 1 {
		t.Fatalf("expected 1 album in library 1, got %d", counts[artist.ID])
	}
}

func TestArtistsWithMBIDAndStamp(t *testing.T) {
	st := testStore(t)
	_ = st.Transaction(func(tx *store.Store) error {
		_, e := tx.FindOrCreateArtists([]string{"A", "B"}, []string{"mbid-a", ""})
		return e
	})
	withMBID, err := st.ArtistsWithMBID()
	if err != nil {
		t.Fatalf("ArtistsWithMBID: %v", err)
	}
	if len(withMBID) != 1 || withMBID[0].MBArtistID != "mbid-a" {
		t.Fatalf("expected 1 artist with MBID, got %+v", withMBID)
	}
	now := time.Now()
	if err := st.SetArtistImageFetchedAt(withMBID[0].ID, now); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	got, _, _ := st.GetArtist(withMBID[0].ID)
	if got.LastImageFetchAt == nil {
		t.Fatal("LastImageFetchAt not stamped")
	}
}

func TestSearchArtistsByLibrary(t *testing.T) {
	s := testStore(t)
	db := s.DB()

	lib1 := model.Library{Name: "L1", Path: "/l1"}
	lib2 := model.Library{Name: "L2", Path: "/l2"}
	db.Create(&lib1)
	db.Create(&lib2)

	a1 := model.Artist{Name: "Alpha", NameNorm: "alpha"}
	a2 := model.Artist{Name: "Alphonse", NameNorm: "alphonse"}
	db.Create(&a1)
	db.Create(&a2)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "x"}
	db.Create(&album)
	t1 := model.Track{AlbumID: album.ID, LibraryID: lib1.ID, Filename: "1.mp3", FilePath: "/l1/1.mp3"}
	t2 := model.Track{AlbumID: album.ID, LibraryID: lib2.ID, Filename: "2.mp3", FilePath: "/l2/2.mp3"}
	db.Create(&t1)
	db.Create(&t2)
	_ = db.Model(&t1).Association("Artists").Replace([]*model.Artist{&a1})
	_ = db.Model(&t2).Association("Artists").Replace([]*model.Artist{&a2})

	id1 := lib1.ID
	got, err := s.SearchArtists("alph", 10, 0, &store.SearchFilter{LibraryID: &id1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Alpha" {
		t.Fatalf("expected Alpha only, got %+v", got)
	}
}

func TestSetArtistMBID(t *testing.T) {
	s := testStore(t)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	artist := artists[0]

	if err := s.SetArtistImageFetchedAt(artist.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	newMbid := "5b11f4ce-a62d-471e-81fc-a69a8278c7da"
	if err := s.SetArtistMBID(artist.ID, newMbid); err != nil {
		t.Fatalf("SetArtistMBID: %v", err)
	}

	updated, _, err := s.GetArtist(artist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MBArtistID != newMbid {
		t.Fatalf("expected MBArtistID %q, got %q", newMbid, updated.MBArtistID)
	}
	if updated.LastImageFetchAt != nil {
		t.Fatal("expected LastImageFetchAt to be cleared")
	}
}

func TestSetArtistMBIDClear(t *testing.T) {
	s := testStore(t)
	artists, err := s.FindOrCreateArtists([]string{"Nirvana"}, []string{"old-mbid"})
	if err != nil {
		t.Fatal(err)
	}
	artist := artists[0]

	if err := s.SetArtistMBID(artist.ID, ""); err != nil {
		t.Fatalf("SetArtistMBID: %v", err)
	}

	updated, _, err := s.GetArtist(artist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MBArtistID != "" {
		t.Fatalf("expected MBArtistID cleared, got %q", updated.MBArtistID)
	}
}

func TestFindOrCreateArtists_TagOverwritesDifferingMBID(t *testing.T) {
	s := testStore(t)
	// Create with an initial MBID and a set image-fetch timestamp.
	err := s.Transaction(func(tx *store.Store) error {
		_, e := tx.FindOrCreateArtists([]string{"Muse"}, []string{"mbid-old"})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := s.SetArtistImageFetchedAt(mustArtistID(t, s, "Muse"), now); err != nil {
		t.Fatal(err)
	}
	// Rescan finds the same artist with a corrected MBID.
	var got []*model.Artist
	err = s.Transaction(func(tx *store.Store) error {
		var e error
		got, e = tx.FindOrCreateArtists([]string{"Muse"}, []string{"mbid-new"})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].MBArtistID != "mbid-new" {
		t.Fatalf("expected overwrite to mbid-new, got %q", got[0].MBArtistID)
	}
	if got[0].LastImageFetchAt != nil {
		t.Fatalf("expected LastImageFetchAt reset to nil, got %v", got[0].LastImageFetchAt)
	}
}

func mustArtistID(t *testing.T, s *store.Store, name string) uint {
	t.Helper()
	var artist model.Artist
	if err := s.DB().Where("name = ?", name).First(&artist).Error; err != nil {
		t.Fatalf("artist %q not found: %v", name, err)
	}
	return artist.ID
}

// seedArtistTrack creates an artist with one track in the given library,
// credited both on the track and on its album, mirroring a regular
// (non-compilation) scan result.
func seedArtistTrack(t *testing.T, s *store.Store, libID uint, artistName, file string) *model.Artist {
	t.Helper()
	artists, err := s.FindOrCreateArtists([]string{artistName}, nil)
	if err != nil {
		t.Fatal(err)
	}
	albumName := "Album of " + artistName
	ident := store.AlbumIdentity{Name: albumName, NameNorm: unidecode.Normalize(albumName), AlbumArtistNorm: artistName, MBReleaseID: ""}
	album, err := s.FindOrCreateAlbum(ident)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DB().Model(album).Association("Artists").Replace(artists); err != nil {
		t.Fatal(err)
	}
	track := &model.Track{AlbumID: album.ID, LibraryID: libID, Title: file, FilePath: file, Filename: file}
	if err := s.UpsertTrack(track, artists, nil); err != nil {
		t.Fatal(err)
	}
	return artists[0]
}

// seedGuestAppearance creates an album owned by ownerName (album_artists credit)
// with one track in libID credited to guestName only (track_artists), mirroring
// how the scanner records a featured artist or compilation contributor.
func seedGuestAppearance(t *testing.T, s *store.Store, libID uint, albumName, ownerName, guestName, file string) (owner, guest *model.Artist, album *model.Album) {
	t.Helper()
	db := s.DB()
	owners, err := s.FindOrCreateArtists([]string{ownerName}, nil)
	if err != nil {
		t.Fatal(err)
	}
	guests, err := s.FindOrCreateArtists([]string{guestName}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ident := store.AlbumIdentity{Name: albumName, NameNorm: unidecode.Normalize(albumName), AlbumArtistNorm: ownerName, MBReleaseID: ""}
	album, err = s.FindOrCreateAlbum(ident)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(album).Association("Artists").Replace(owners); err != nil {
		t.Fatal(err)
	}
	track := &model.Track{AlbumID: album.ID, LibraryID: libID, Title: file, FilePath: file, Filename: file}
	if err := s.UpsertTrack(track, guests, nil); err != nil {
		t.Fatal(err)
	}
	return owners[0], guests[0], album
}

func TestGetArtistsExcludesTrackOnlyGuestArtists(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L1", Path: "/l1"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	seedGuestAppearance(t, s, lib.ID, "Fired Up", "Alesha Dixon", "Asher D", "/l1/1.mp3")

	artists, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 || artists[0].Name != "Alesha Dixon" {
		t.Fatalf("expected only the album artist in the index, got %+v", artists)
	}

	filtered, err := s.GetArtists(&store.ArtistsFilter{LibraryID: &lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Name != "Alesha Dixon" {
		t.Fatalf("expected only the album artist in the filtered index, got %+v", filtered)
	}
}

func TestGetArtistsByLibraryIncludesAlbumArtistWithoutTrackCredits(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L1", Path: "/l1"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	// Compilation shape: album credited to Various Artists, tracks credited to guests.
	seedGuestAppearance(t, s, lib.ID, "Cyberpunk 2077", "Various Artists", "P.T. Adamczyk", "/l1/1.mp3")

	filtered, err := s.GetArtists(&store.ArtistsFilter{LibraryID: &lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Name != "Various Artists" {
		t.Fatalf("expected Various Artists in the filtered index, got %+v", filtered)
	}
}

func TestGetArtistReturnsAlbumsTheArtistAppearsOn(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L1", Path: "/l1"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	_, guest, album := seedGuestAppearance(t, s, lib.ID, "Fired Up", "Alesha Dixon", "Asher D", "/l1/1.mp3")

	_, albums, err := s.GetArtist(guest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 || albums[0].ID != album.ID {
		t.Fatalf("expected the guest's appearance album, got %+v", albums)
	}
}

func TestGetArtistAlbumCountsIncludesAppearances(t *testing.T) {
	s := testStore(t)
	lib := &model.Library{Name: "L1", Path: "/l1"}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	owner, guest, _ := seedGuestAppearance(t, s, lib.ID, "Fired Up", "Alesha Dixon", "Asher D", "/l1/1.mp3")

	counts, err := s.GetArtistAlbumCounts(nil)
	if err != nil {
		t.Fatal(err)
	}
	if counts[owner.ID] != 1 {
		t.Fatalf("expected owner album count 1, got %d", counts[owner.ID])
	}
	if counts[guest.ID] != 1 {
		t.Fatalf("expected guest appearance count 1, got %d", counts[guest.ID])
	}

	filtered, err := s.GetArtistAlbumCounts(&store.ArtistsFilter{LibraryID: &lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if filtered[guest.ID] != 1 {
		t.Fatalf("expected guest appearance count 1 in library filter, got %d", filtered[guest.ID])
	}
}

func TestGetArtistsExcludesHiddenLibraries(t *testing.T) {
	s := testStore(t)
	vis := &model.Library{Name: "Vis", Path: "/vis"}
	hid := &model.Library{Name: "Hid", Path: "/hid", HideArtists: true}
	if err := s.CreateLibrary(vis); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(hid); err != nil {
		t.Fatal(err)
	}
	seedArtistTrack(t, s, vis.ID, "Visible Artist", "/vis/a.mp3")
	seedArtistTrack(t, s, hid.ID, "Hidden Artist", "/hid/b.mp3")

	artists, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 || artists[0].Name != "Visible Artist" {
		t.Fatalf("expected only Visible Artist, got %+v", artists)
	}
}

func TestGetArtistsKeepsArtistsSharedWithVisibleLibrary(t *testing.T) {
	s := testStore(t)
	vis := &model.Library{Name: "Vis", Path: "/vis"}
	hid := &model.Library{Name: "Hid", Path: "/hid", HideArtists: true}
	if err := s.CreateLibrary(vis); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLibrary(hid); err != nil {
		t.Fatal(err)
	}
	seedArtistTrack(t, s, vis.ID, "Shared Artist", "/vis/a.mp3")
	seedArtistTrack(t, s, hid.ID, "Shared Artist", "/hid/b.mp3")

	artists, err := s.GetArtists(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 || artists[0].Name != "Shared Artist" {
		t.Fatalf("expected Shared Artist to stay visible, got %+v", artists)
	}
}

func TestGetArtistsFilterByHiddenLibraryIsEmpty(t *testing.T) {
	s := testStore(t)
	hid := &model.Library{Name: "Hid", Path: "/hid", HideArtists: true}
	if err := s.CreateLibrary(hid); err != nil {
		t.Fatal(err)
	}
	seedArtistTrack(t, s, hid.ID, "Hidden Artist", "/hid/b.mp3")

	artists, err := s.GetArtists(&store.ArtistsFilter{LibraryID: &hid.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 0 {
		t.Fatalf("expected empty index for hidden library, got %+v", artists)
	}
}
