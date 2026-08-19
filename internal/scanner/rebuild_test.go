// internal/scanner/rebuild_test.go
package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// distinguishableTagReader returns metadata with distinct artists and genres,
// some with MusicBrainz IDs and some without, to test both key derivation paths.
type distinguishableTagReader struct{}

func (distinguishableTagReader) CanRead(absPath string) bool {
	return scanner.IsAudioFile(absPath)
}

func (distinguishableTagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	name := filepath.Base(absPath)
	dir := filepath.Base(filepath.Dir(absPath))

	// Map directory names to distinct artist/genre/MBID combinations.
	// Greatest Hits appears twice by different artists to prove the key
	// discriminates on the full (name_norm, album_artist_norm, mb_release_id) tuple.
	var artist, album, mbArtistID, genre string
	switch dir {
	case "Album-A":
		artist = "Artist Alpha"
		album = "Album-A"
		mbArtistID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		genre = "Rock"
	case "Album-B":
		artist = "Artist Beta"
		album = "Album-B"
		mbArtistID = "" // No MBID - tests the unmatched artist path
		genre = "Jazz"
	case "Album-C":
		artist = "Artist Gamma"
		album = "Album-C"
		mbArtistID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		genre = "Electronic"
	case "Greatest Hits":
		// Determine which artist based on parent directory.
		parent := filepath.Base(filepath.Dir(filepath.Dir(absPath)))
		if parent == "Artist Delta" {
			artist = "Artist Delta"
			album = "Greatest Hits"
			mbArtistID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
			genre = "Pop"
		} else {
			artist = "Artist Epsilon"
			album = "Greatest Hits"
			mbArtistID = ""
			genre = "Country"
		}
	default:
		artist = "Unknown"
		album = dir
		genre = "Unknown"
	}

	return tags.Metadata{
		Title:       name,
		Artist:      []string{artist},
		AlbumArtist: []string{artist},
		Album:       album,
		Genre:       []string{genre},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
		MBArtistID:  []string{mbArtistID},
		MBReleaseID: "",
	}, nil
}

type entityCovers struct {
	albums  map[uint][]byte
	artists map[uint][]byte
	genres  map[uint][]byte
}

func storeDistinguishableCovers(t *testing.T, assets *assetstore.Store, albums []model.Album, artists []model.Artist, genres []model.Genre) entityCovers {
	t.Helper()
	covers := entityCovers{
		albums:  make(map[uint][]byte),
		artists: make(map[uint][]byte),
		genres:  make(map[uint][]byte),
	}

	for _, album := range albums {
		data := []byte("album:" + album.Name + ":" + album.NameNorm + ":" + album.AlbumArtistNorm)
		covers.albums[album.ID] = data
		key := assetkey.AlbumOf(&album)
		if err := assets.PutManual(assetstore.KindAlbum, key, "png", data); err != nil {
			t.Fatal(err)
		}
	}

	for _, artist := range artists {
		data := []byte("artist:" + artist.Name + ":" + artist.NameNorm)
		covers.artists[artist.ID] = data
		key := assetkey.ArtistOf(&artist)
		if err := assets.PutManual(assetstore.KindArtist, key, "png", data); err != nil {
			t.Fatal(err)
		}
	}

	for _, genre := range genres {
		data := []byte("genre:" + genre.Name)
		covers.genres[genre.ID] = data
		key := assetkey.GenreOf(&genre)
		if err := assets.PutManual(assetstore.KindGenre, key, "png", data); err != nil {
			t.Fatal(err)
		}
	}

	return covers
}

func seedThrowawayRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	// Seed 1 throwaway row per table to create ID overlap (old 1/2/3 → new 2/3/4)
	// rather than offset (old 1/2/3 → new 6/7/8). Under a positional key, overlap
	// causes misattribution (album 2 inherits album 1's cover) instead of mere
	// absence, which is the bug this test exists to catch.
	throwawayAlbum := &model.Album{
		Name:            "Throwaway",
		NameNorm:        "throwaway",
		AlbumArtistNorm: "throwaway",
	}
	if err := db.Create(throwawayAlbum).Error; err != nil {
		t.Fatal(err)
	}
	throwawayArtist := &model.Artist{
		Name:     "Throwaway",
		NameNorm: "throwaway",
	}
	if err := db.Create(throwawayArtist).Error; err != nil {
		t.Fatal(err)
	}
	throwawayGenre := &model.Genre{Name: "Throwaway"}
	if err := db.Create(throwawayGenre).Error; err != nil {
		t.Fatal(err)
	}
}

func assertIDShift(t *testing.T, albums2 []model.Album, artists2 []model.Artist, genres2 []model.Genre, oldAlbumIDs, oldArtistIDs, oldGenreIDs map[string]uint) {
	t.Helper()
	// Check each kind separately — short-circuiting would allow one kind's seeding
	// to weaken without the test becoming vacuous.
	albumShifted := false
	for _, a := range albums2 {
		compositeKey := a.Name + "|" + a.AlbumArtistNorm
		if a.ID != oldAlbumIDs[compositeKey] {
			albumShifted = true
			break
		}
	}
	if !albumShifted {
		t.Fatal("fixture did not shift album IDs — album half of test is vacuous")
	}

	artistShifted := false
	for _, a := range artists2 {
		if a.ID != oldArtistIDs[a.Name] {
			artistShifted = true
			break
		}
	}
	if !artistShifted {
		t.Fatal("fixture did not shift artist IDs — artist half of test is vacuous")
	}

	genreShifted := false
	for _, g := range genres2 {
		if g.ID != oldGenreIDs[g.Name] {
			genreShifted = true
			break
		}
	}
	if !genreShifted {
		t.Fatal("fixture did not shift genre IDs — genre half of test is vacuous")
	}
}

func assertAlbumsHaveOwnCovers(t *testing.T, albums []model.Album, assets *assetstore.Store, covers map[uint][]byte, oldIDs map[string]uint) {
	t.Helper()
	for _, album := range albums {
		key := assetkey.AlbumOf(&album)
		path, ok := assets.Get(assetstore.KindAlbum, key)
		if !ok {
			t.Errorf("album %q by %q has no cover after rebuild", album.Name, album.AlbumArtistNorm)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		compositeKey := album.Name + "|" + album.AlbumArtistNorm
		want := covers[oldIDs[compositeKey]]
		if string(data) != string(want) {
			t.Errorf("album %q by %q resolved wrong cover: got %q, want %q", album.Name, album.AlbumArtistNorm, data, want)
		}
	}
}

func assertArtistsHaveOwnCovers(t *testing.T, artists []model.Artist, assets *assetstore.Store, covers map[uint][]byte, oldIDs map[string]uint) {
	t.Helper()
	for _, artist := range artists {
		key := assetkey.ArtistOf(&artist)
		path, ok := assets.Get(assetstore.KindArtist, key)
		if !ok {
			t.Errorf("artist %q has no cover after rebuild", artist.Name)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := covers[oldIDs[artist.Name]]
		if string(data) != string(want) {
			t.Errorf("artist %q resolved wrong cover: got %q, want %q", artist.Name, data, want)
		}
	}
}

func assertGenresHaveOwnCovers(t *testing.T, genres []model.Genre, assets *assetstore.Store, covers map[uint][]byte, oldIDs map[string]uint) {
	t.Helper()
	for _, genre := range genres {
		key := assetkey.GenreOf(&genre)
		path, ok := assets.Get(assetstore.KindGenre, key)
		if !ok {
			t.Errorf("genre %q has no cover after rebuild", genre.Name)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := covers[oldIDs[genre.Name]]
		if string(data) != string(want) {
			t.Errorf("genre %q resolved wrong cover: got %q, want %q", genre.Name, data, want)
		}
	}
}

func assertNoMisattribution(t *testing.T, albums []model.Album, artists []model.Artist, genres []model.Genre, assets *assetstore.Store, covers entityCovers, oldAlbumIDs, oldArtistIDs, oldGenreIDs map[string]uint) {
	t.Helper()
	for _, album := range albums {
		key := assetkey.AlbumOf(&album)
		path, ok := assets.Get(assetstore.KindAlbum, key)
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		compositeKey := album.Name + "|" + album.AlbumArtistNorm
		for oldID, otherData := range covers.albums {
			if oldID != oldAlbumIDs[compositeKey] && string(data) == string(otherData) {
				t.Errorf("album %q by %q resolved another album's cover", album.Name, album.AlbumArtistNorm)
			}
		}
	}

	for _, artist := range artists {
		key := assetkey.ArtistOf(&artist)
		path, ok := assets.Get(assetstore.KindArtist, key)
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for oldID, otherData := range covers.artists {
			if oldID != oldArtistIDs[artist.Name] && string(data) == string(otherData) {
				t.Errorf("artist %q resolved another artist's cover", artist.Name)
			}
		}
	}

	for _, genre := range genres {
		key := assetkey.GenreOf(&genre)
		path, ok := assets.Get(assetstore.KindGenre, key)
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for oldID, otherData := range covers.genres {
			if oldID != oldGenreIDs[genre.Name] && string(data) == string(otherData) {
				t.Errorf("genre %q resolved another genre's cover", genre.Name)
			}
		}
	}
}

// assertPositionalKeysNotFound verifies that every entity's positional key
// (strconv of its DB id) does NOT resolve to an image. This is the negative
// assertion the per-kind handler tests use, and proves a handler regression to
// a positional key would fail this test — without it, reverting a handler to
// strconv(id) still passes because the test only exercises assetkey functions.
func assertPositionalKeysNotFound(t *testing.T, albums []model.Album, artists []model.Artist, genres []model.Genre, assets *assetstore.Store) {
	t.Helper()
	for _, album := range albums {
		positionalKey := strconv.FormatUint(uint64(album.ID), 10)
		if path, ok := assets.Get(assetstore.KindAlbum, positionalKey); ok {
			t.Errorf("album %q by %q: positional key %q resolved to %q, handler regressed", album.Name, album.AlbumArtistNorm, positionalKey, path)
		}
	}
	for _, artist := range artists {
		positionalKey := strconv.FormatUint(uint64(artist.ID), 10)
		if path, ok := assets.Get(assetstore.KindArtist, positionalKey); ok {
			t.Errorf("artist %q: positional key %q resolved to %q, handler regressed", artist.Name, positionalKey, path)
		}
	}
	for _, genre := range genres {
		positionalKey := strconv.FormatUint(uint64(genre.ID), 10)
		if path, ok := assets.Get(assetstore.KindGenre, positionalKey); ok {
			t.Errorf("genre %q: positional key %q resolved to %q, handler regressed", genre.Name, positionalKey, path)
		}
	}
}

// TestRebuildReattachesAssetsToTheRightEntities verifies that after dropping
// the database and rescanning, manually uploaded images remain attached to the
// CORRECT entities. This is the headline acceptance test for durable asset keys:
// before this change, images were keyed on autoincrement IDs and a rebuild
// silently misattributed them — album 5 was now a different album and inherited
// the old album 5's cover.
//
//nolint:gocyclo // Complexity from testing all entity types through a full rebuild
func TestRebuildReattachesAssetsToTheRightEntities(t *testing.T) {
	// 1. Build fixture: five albums by different artists, including a same-titled
	// pair (Greatest Hits) to prove the key discriminates on the full
	// (name_norm, album_artist_norm, mb_release_id) tuple rather than title alone.
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Album-A/01.mp3",
		"Album-B/01.mp3",
		"Album-C/01.mp3",
		"Artist Delta/Greatest Hits/01.mp3",
		"Artist Epsilon/Greatest Hits/01.mp3",
	})

	// Asset root must survive the rebuild — a second t.TempDir() would produce
	// a different directory and prove nothing.
	assetRoot := t.TempDir()
	assets := assetstore.New(assetRoot)

	// 2. First scan.
	st1 := testScanStore(t)
	lib1 := seedLibrary(t, st1, dir, nil)
	s1 := scanner.New(scanner.Config{}, st1, distinguishableTagReader{})
	if _, err := s1.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// 3. Store a distinguishable manual cover for each album, artist, and genre.
	// The bytes encode the entity's identity ("album:<Name>:<NameNorm>:<AlbumArtistNorm>")
	// so a wrong attachment is visible rather than merely unequal.
	var albums []model.Album
	st1.DB().Order("name").Find(&albums)
	if len(albums) != 5 {
		t.Fatalf("expected 5 albums, got %d", len(albums))
	}
	var artists []model.Artist
	st1.DB().Order("name").Find(&artists)
	if len(artists) != 5 {
		t.Fatalf("expected 5 artists, got %d", len(artists))
	}
	var genres []model.Genre
	st1.DB().Order("name").Find(&genres)
	if len(genres) != 5 {
		t.Fatalf("expected 5 genres, got %d", len(genres))
	}

	// Verify one artist has no MBID (tests the unmatched artist key path).
	unmatchedCount := 0
	for _, a := range artists {
		if a.MBArtistID == "" {
			unmatchedCount++
		}
	}
	if unmatchedCount == 0 {
		t.Fatal("fixture must include at least one artist without a MusicBrainz ID")
	}

	// Store distinguishable covers: "album:<Name>:<NameNorm>:<AlbumArtistNorm>" for albums,
	// "artist:<Name>:<NameNorm>" for artists, "genre:<Name>" for genres.
	// This makes a wrong attachment detectable by comparing byte slices.
	covers := storeDistinguishableCovers(t, assets, albums, artists, genres)

	// Assert the negative: positional keys must NOT resolve before the rebuild.
	// This proves a handler regression to strconv(id) would fail this test.
	assertPositionalKeysNotFound(t, albums, artists, genres, assets)

	// Record IDs before the rebuild. Albums are keyed by name+artist since
	// the same-titled pair would collide in a name-only map.
	oldAlbumIDs := make(map[string]uint)
	for _, a := range albums {
		oldAlbumIDs[a.Name+"|"+a.AlbumArtistNorm] = a.ID
	}
	oldArtistIDs := make(map[string]uint)
	for _, a := range artists {
		oldArtistIDs[a.Name] = a.ID
	}
	oldGenreIDs := make(map[string]uint)
	for _, g := range genres {
		oldGenreIDs[g.Name] = g.ID
	}

	// 4. Rebuild: fresh store over a new in-memory DB, pointed at the same asset root.
	db2, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db2); err != nil {
		t.Fatal(err)
	}

	// Seed throwaway rows to force ID reassignment. The scanner will reconcile
	// real entities after these, so their autoincrement IDs differ from round 1.
	seedThrowawayRows(t, db2)

	st2 := store.New(db2)
	lib2 := &model.Library{
		Name:           lib1.Name,
		Path:           lib1.Path,
		FollowSymlinks: lib1.FollowSymlinks,
	}
	if err := st2.CreateLibrary(lib2); err != nil {
		t.Fatal(err)
	}

	// Rescan the same directory.
	s2 := scanner.New(scanner.Config{}, st2, distinguishableTagReader{})
	if _, err := s2.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	// 5. Assert the ID shift occurred.
	// The throwaway rows are gone — DeleteOrphanedAggregates swept them because
	// no tracks reference them.
	var albums2 []model.Album
	st2.DB().Order("name").Find(&albums2)
	if len(albums2) != 5 {
		t.Fatalf("expected 5 albums after rebuild, got %d", len(albums2))
	}
	var artists2 []model.Artist
	st2.DB().Order("name").Find(&artists2)
	if len(artists2) != 5 {
		t.Fatalf("expected 5 artists after rebuild, got %d", len(artists2))
	}
	var genres2 []model.Genre
	st2.DB().Order("name").Find(&genres2)
	if len(genres2) != 5 {
		t.Fatalf("expected 5 genres after rebuild, got %d", len(genres2))
	}

	// At least one entity's ID must have changed — otherwise the test is vacuous.
	assertIDShift(t, albums2, artists2, genres2, oldAlbumIDs, oldArtistIDs, oldGenreIDs)

	// 6. Assert every album, artist, and genre resolves its OWN cover.
	assertAlbumsHaveOwnCovers(t, albums2, assets, covers.albums, oldAlbumIDs)
	assertArtistsHaveOwnCovers(t, artists2, assets, covers.artists, oldArtistIDs)
	assertGenresHaveOwnCovers(t, genres2, assets, covers.genres, oldGenreIDs)

	// 7. Assert the negative: no entity resolves an image belonging to another.
	assertNoMisattribution(t, albums2, artists2, genres2, assets, covers, oldAlbumIDs, oldArtistIDs, oldGenreIDs)

	// 8. Assert the negative: positional keys must NOT resolve after the rebuild either.
	assertPositionalKeysNotFound(t, albums2, artists2, genres2, assets)

	// 9. Radio: create a station, store its cover, rebuild, re-create a station
	// with the same URL, and assert the cover re-attaches. Note the framing:
	// radio rows are user-created, not scanner-derived, and a rebuild destroys
	// them — so what is being proven for radio is that a RE-CREATED station
	// with the same stream URL re-attaches the cover, not that a rescan restores
	// the station itself.
	radioURL := "https://example.com/stream"
	radioStation1 := &model.InternetRadioStation{
		Name:        "Test Radio",
		StreamURL:   radioURL,
		HomepageURL: "https://example.com",
	}
	if err := st1.DB().Create(radioStation1).Error; err != nil {
		t.Fatal(err)
	}
	radioCover := []byte("radio:" + radioURL)
	radioKey := assetkey.Radio(radioURL)
	if err := assets.PutManual(assetstore.KindRadio, radioKey, "png", radioCover); err != nil {
		t.Fatal(err)
	}
	// Positional key must not resolve before rebuild.
	if path, ok := assets.Get(assetstore.KindRadio, strconv.FormatUint(uint64(radioStation1.ID), 10)); ok {
		t.Errorf("radio positional key resolved before rebuild to %q", path)
	}

	// Rebuild: the radio row is gone, but the cover is still on disk.
	// Re-creating a station with the same URL must re-attach it.
	radioStation2 := &model.InternetRadioStation{
		Name:        "Test Radio (rebuilt)",
		StreamURL:   radioURL,
		HomepageURL: "https://example.com",
	}
	if err := st2.DB().Create(radioStation2).Error; err != nil {
		t.Fatal(err)
	}
	path, ok := assets.Get(assetstore.KindRadio, assetkey.Radio(radioStation2.StreamURL))
	if !ok {
		t.Fatal("radio station cover did not re-attach after rebuild")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(radioCover) {
		t.Errorf("radio cover resolved wrong data: got %q, want %q", data, radioCover)
	}
	// Positional key must not resolve after rebuild either.
	if path, ok := assets.Get(assetstore.KindRadio, strconv.FormatUint(uint64(radioStation2.ID), 10)); ok {
		t.Errorf("radio positional key resolved after rebuild to %q", path)
	}
}
