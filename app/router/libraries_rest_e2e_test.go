package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestALibraryCreatedThroughTheAPINarrowsRest drives the whole chain in one
// request sequence — /api/v0 normalize → store → store.ScopeOf → /rest — on the
// real router. Both halves are covered separately (the handler round-trips
// normalized filters, and the subsonic package narrows store-seeded libraries),
// but only together do they prove that what the API NORMALIZES on the way in is
// what the compiler MATCHES on the way out: " Music " and "FLAC" are trimmed and
// lowercased because the server owns those vocabularies, while the genre "Rock "
// keeps its trailing space because it is compared with the tag the scanner
// recorded. Normalizing the genre too — or failing to normalize the folder —
// would leave a library that saves cleanly and then selects nothing, which
// neither layer's own tests would notice.
func TestALibraryCreatedThroughTheAPINarrowsRest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)

	// "Rock " carries a trailing space, the way a sloppy tagger writes it and
	// the way the scanner stores it — verbatim.
	rock := model.Genre{Name: "Rock "}
	// selected: a flac in Music tagged "Rock " — everything the library asks
	// for. excluded: the same scan folder, the wrong format, no genre.
	selected := model.Album{Name: "Selected", NameNorm: "selected", AlbumArtistNorm: "someone"}
	excluded := model.Album{Name: "Excluded", NameNorm: "excluded", AlbumArtistNorm: "someone"}
	seedRows(t, db, &rock, &selected, &excluded)
	selectedTrack := model.Track{AlbumID: selected.ID, ScanFolder: "Music", Suffix: "flac", Filename: "1.flac", FilePath: "/music/1.flac", Title: "One"}
	excludedTrack := model.Track{AlbumID: excluded.ID, ScanFolder: "Music", Suffix: "mp3", Filename: "2.mp3", FilePath: "/music/2.mp3", Title: "Two"}
	seedRows(t, db, &selectedTrack, &excludedTrack)
	if err := db.Model(&selectedTrack).Association("Genres").Replace([]*model.Genre{&rock}); err != nil {
		t.Fatal(err)
	}

	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Music", Path: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Cfg{
		Store:       s,
		DataDir:     t.TempDir(),
		ScanFolders: set,
		AuthMethod:  "none",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := `{"name":"Lossless Rock","filters":[` +
		`{"field":"scan_folder","values":[" Music "]},` +
		`{"field":"format","values":["FLAC"]},` +
		`{"field":"genre","values":["Rock "]}` +
		`]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v0/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create library: expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var created struct {
		ID      uint                  `json:"id"`
		Filters []model.LibraryFilter `json:"filters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assertFilters(t, created.Filters, []model.LibraryFilter{
		{Field: model.FilterScanFolder, Values: []string{"Music"}},
		{Field: model.FilterFormat, Values: []string{"flac"}},
		{Field: model.FilterGenre, Values: []string{"Rock "}},
	})

	scoped := restAlbumNames(t, h, "musicFolderId="+strconv.FormatUint(uint64(created.ID), 10))
	if len(scoped) != 1 || scoped[0] != "Selected" {
		t.Fatalf("albums in the created library = %v, want only [Selected]", scoped)
	}
	if whole := restAlbumNames(t, h, "size=50"); len(whole) != 2 {
		t.Fatalf("albums without musicFolderId = %v, want both", whole)
	}
}

// seedRows inserts fixture rows, failing the test on the first refusal.
func seedRows(t *testing.T, db *gorm.DB, rows ...any) {
	t.Helper()
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// assertFilters compares filters field by field, so a failure names the entry
// and the exact value — trailing spaces included, which is the whole point.
func assertFilters(t *testing.T, got, want []model.LibraryFilter) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("filters = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Field != want[i].Field || !slices.Equal(got[i].Values, want[i].Values) {
			t.Errorf("filters[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// restAlbumNames asks /rest for an album list and returns the names it answers.
func restAlbumNames(t *testing.T, h http.Handler, query string) []string {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rest/getAlbumList2.view?f=json&type=alphabeticalByName&"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("getAlbumList2 %q: http %d, body=%s", query, w.Code, w.Body.String())
	}
	var envelope struct {
		SubsonicResponse struct {
			Status     string `json:"status"`
			AlbumList2 struct {
				Album []struct {
					Name string `json:"name"`
				} `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("bad body %q: %v", w.Body.String(), err)
	}
	if envelope.SubsonicResponse.Status != "ok" {
		t.Fatalf("getAlbumList2 %q: status %q, body=%s", query, envelope.SubsonicResponse.Status, w.Body.String())
	}
	names := make([]string, 0, len(envelope.SubsonicResponse.AlbumList2.Album))
	for _, al := range envelope.SubsonicResponse.AlbumList2.Album {
		names = append(names, al.Name)
	}
	return names
}
