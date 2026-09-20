package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/go-bumbu/http/problemjson"
)

// TestPreviewCountsMatchTheFilters seeds two albums in different scan
// folders, so a filtered preview and the whole-catalog preview (no filters)
// report genuinely different counts — proving preview actually scopes the
// query rather than always answering the same numbers.
func TestPreviewCountsMatchTheFilters(t *testing.T) {
	_, s, r := newTestHandler(t)
	db := s.DB()
	albumMusic := model.Album{Name: "Music Album", NameNorm: "music album", AlbumArtistNorm: "x"}
	albumBooks := model.Album{Name: "Books Album", NameNorm: "books album", AlbumArtistNorm: "x"}
	if err := db.Create(&albumMusic).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&albumBooks).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Track{AlbumID: albumMusic.ID, ScanFolder: "Music", Suffix: "flac", Filename: "1.flac", FilePath: "/music/1.flac"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Track{AlbumID: albumMusic.ID, ScanFolder: "Music", Suffix: "mp3", Filename: "2.mp3", FilePath: "/music/2.mp3"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Track{AlbumID: albumBooks.ID, ScanFolder: "Books", Suffix: "mp3", Filename: "3.mp3", FilePath: "/books/3.mp3"}).Error; err != nil {
		t.Fatal(err)
	}

	preview := func(body string) (code int, trackCount, albumCount int64) {
		req := httptest.NewRequest("POST", "/libraries/preview", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			return w.Code, 0, 0
		}
		var got struct {
			TrackCount int64 `json:"track_count"`
			AlbumCount int64 `json:"album_count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return w.Code, got.TrackCount, got.AlbumCount
	}

	if code, tracks, albums := preview(`{"filters":[{"field":"scan_folder","values":["Music"]}]}`); code != http.StatusOK || tracks != 2 || albums != 1 {
		t.Fatalf("Music filter: code=%d tracks=%d albums=%d, want 200/2/1", code, tracks, albums)
	}
	// No filters at all: the whole catalog, both albums.
	if code, tracks, albums := preview(`{}`); code != http.StatusOK || tracks != 3 || albums != 2 {
		t.Fatalf("no filters: code=%d tracks=%d albums=%d, want 200/3/2 (the whole catalog)", code, tracks, albums)
	}
}

// TestPreviewRejectsAnInvalidFilter proves preview validates filters through
// the same libraryfilter.Validate as create/update, reporting the exact
// pointer of the problem rather than a generic error.
func TestPreviewRejectsAnInvalidFilter(t *testing.T) {
	_, _, r := newTestHandler(t)
	body := `{"filters":[{"field":"scan_folder","values":["Nope"]}]}`
	req := httptest.NewRequest("POST", "/libraries/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d, body=%s", w.Code, w.Body.String())
	}
	var problem problemjson.ValidationDetails
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) != 1 || problem.Errors[0].Pointer != "/filters/0/values/0" {
		t.Fatalf("expected one error at /filters/0/values/0, got %+v", problem.Errors)
	}
}

func TestPreviewRejectsMalformedJSON(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("POST", "/libraries/preview", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}
