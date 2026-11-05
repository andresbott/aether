package wishlist_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/problems"
	wishlistHandler "github.com/andresbott/aether/app/router/handlers/wishlist"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/http/outbound"
	"github.com/go-bumbu/http/problemjson"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

const (
	mbArtist = "5b11f4ce-a62d-471e-81fc-a69a8278c7da"
	mbGroup  = "1b022e01-4da6-387b-8658-8678046e4cef"
	mbRec    = "9b1b4a2c-3c55-4c5e-8f0e-1a2b3c4d5e6f"
)

type fakeSearcher struct {
	artists   []artistimage.Candidate
	groups    []artistimage.ReleaseGroupCandidate
	songs     []artistimage.RecordingCandidate
	err       error
	lastQuery string
}

func (f *fakeSearcher) Search(_ context.Context, q string, _ int) ([]artistimage.Candidate, error) {
	f.lastQuery = q
	return f.artists, f.err
}

func (f *fakeSearcher) SearchReleaseGroups(_ context.Context, q string, _ int) ([]artistimage.ReleaseGroupCandidate, error) {
	f.lastQuery = q
	return f.groups, f.err
}

func (f *fakeSearcher) SearchRecordings(_ context.Context, q string, _ int) ([]artistimage.RecordingCandidate, error) {
	f.lastQuery = q
	return f.songs, f.err
}

func newTestHandler(t *testing.T, search *fakeSearcher) (*store.Store, *mux.Router) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	r := mux.NewRouter()
	(&wishlistHandler.Handler{Store: s, Search: search, Problems: problems.New(false)}).Routes(r)
	return s, r
}

func do(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, &buf))
	return w
}

type item struct {
	ID           uint    `json:"id"`
	Kind         string  `json:"kind"`
	MBID         string  `json:"mbid"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	Type         string  `json:"type"`
	Note         string  `json:"note"`
	InLibrary    bool    `json:"in_library"`
	MatchID      uint    `json:"match_id"`
	MatchAlbumID uint    `json:"match_album_id"`
	FoundAt      *string `json:"found_at"`
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return out
}

func TestAddListGetUpdateDelete(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{})
	al := model.Album{Name: "Nevermind", NameNorm: "nevermind", AlbumArtistNorm: "nirvana", MBReleaseGroupID: mbGroup}
	if err := s.DB().Create(&al).Error; err != nil {
		t.Fatal(err)
	}

	w := do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": []map[string]string{
		{"kind": "album", "mbid": strings.ToUpper(mbGroup), "title": " Nevermind ", "artist": "Nirvana"},
		{"kind": "artist", "mbid": mbArtist, "title": "Nirvana"},
	}})
	if w.Code != http.StatusOK {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	added := decode[struct{ Items []item }](t, w).Items
	if len(added) != 2 {
		t.Fatalf("want 2 items, got %d", len(added))
	}
	if added[0].MBID != mbGroup || added[0].Title != "Nevermind" {
		t.Errorf("mbid must be lowercased and title trimmed: %+v", added[0])
	}
	if !added[0].InLibrary || added[0].MatchID != al.ID || added[0].FoundAt == nil {
		t.Errorf("album already in the catalog must come back found: %+v", added[0])
	}
	if added[1].InLibrary || added[1].FoundAt != nil {
		t.Errorf("artist is not in the catalog: %+v", added[1])
	}

	w = do(t, r, http.MethodGet, "/wishlist", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	if got := decode[struct{ Items []item }](t, w).Items; len(got) != 2 {
		t.Fatalf("list: want 2, got %d", len(got))
	}

	id := strconv.FormatUint(uint64(added[1].ID), 10)
	w = do(t, r, http.MethodPut, "/wishlist/"+id, map[string]string{"note": "  the 1989 one "})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if got := decode[item](t, w); got.Note != "the 1989 one" {
		t.Fatalf("note: %+v", got)
	}
	w = do(t, r, http.MethodGet, "/wishlist/"+id, nil)
	if got := decode[item](t, w); got.Note != "the 1989 one" || got.Kind != "artist" {
		t.Fatalf("get: %+v", got)
	}

	if w = do(t, r, http.MethodDelete, "/wishlist/"+id, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if w = do(t, r, http.MethodDelete, "/wishlist/"+id, nil); w.Code != http.StatusNotFound {
		t.Fatalf("second delete: %d", w.Code)
	}
	if w = do(t, r, http.MethodGet, "/wishlist/"+id, nil); w.Code != http.StatusNotFound {
		t.Fatalf("get deleted: %d", w.Code)
	}
}

func TestAddManualEntry(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{})
	w := do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": []map[string]string{
		{"kind": "album", "title": " Unreleased demos ", "note": " the cassette from the 1988 show "},
	}})
	if w.Code != http.StatusOK {
		t.Fatalf("manual add: %d %s", w.Code, w.Body)
	}
	got := decode[struct{ Items []item }](t, w).Items
	if len(got) != 1 || got[0].MBID != "" || got[0].Title != "Unreleased demos" ||
		got[0].Note != "the cassette from the 1988 show" || got[0].InLibrary {
		t.Fatalf("unexpected %+v", got)
	}

	w = do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": []map[string]string{
		{"kind": "album", "title": "x", "note": strings.Repeat("n", wishlistHandler.MaxNoteLen+1)},
	}})
	body := decode[problemjson.ValidationDetails](t, w)
	if w.Code != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0].Pointer != "/items/0/note" {
		t.Fatalf("long note: %d %+v", w.Code, body.Errors)
	}
}

func TestAddValidationItemizesEveryProblem(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{})
	w := do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": []map[string]string{
		{"kind": "album", "mbid": mbGroup, "title": "ok"},
		{"kind": "playlist", "mbid": "not-an-id", "title": "  ", "artist": strings.Repeat("x", 501)},
	}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
	body := decode[problemjson.ValidationDetails](t, w)
	var pointers []string
	for _, e := range body.Errors {
		pointers = append(pointers, e.Pointer)
	}
	want := "/items/1/kind,/items/1/mbid,/items/1/title,/items/1/artist"
	if got := strings.Join(pointers, ","); got != want {
		t.Fatalf("pointers = %s, want %s", got, want)
	}
}

func TestAddRejectsEmptyAndOversizedBatches(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{})
	if w := do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": []any{}}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty: want 422, got %d", w.Code)
	}
	many := make([]map[string]string, wishlistHandler.MaxAddItems+1)
	for i := range many {
		many[i] = map[string]string{"kind": "song", "mbid": mbRec, "title": "x"}
	}
	if w := do(t, r, http.MethodPost, "/wishlist", map[string]any{"items": many}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized: want 422, got %d", w.Code)
	}
	if w := do(t, r, http.MethodPost, "/wishlist", "{not json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed: want 400, got %d", w.Code)
	}
}

func TestUpdateEveryFieldOfAManualEntry(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{})
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, Title: "Radio sesion"}})
	path := "/wishlist/" + strconv.FormatUint(uint64(items[0].ID), 10)

	w := do(t, r, http.MethodPut, path, map[string]string{"kind": "album", "title": " Radio session ", "note": "BBC"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	got := decode[item](t, w)
	if got.Kind != "album" || got.Title != "Radio session" || got.Note != "BBC" {
		t.Fatalf("unexpected %+v", got)
	}

	w = do(t, r, http.MethodPut, path, map[string]string{"kind": "genre", "title": "  "})
	body := decode[problemjson.ValidationDetails](t, w)
	if w.Code != http.StatusUnprocessableEntity || len(body.Errors) != 2 {
		t.Fatalf("want title and kind itemized, got %d %+v", w.Code, body.Errors)
	}
}

func TestUpdateRefusesTitleOfAPick(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{})
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"}})
	path := "/wishlist/" + strconv.FormatUint(uint64(items[0].ID), 10)

	w := do(t, r, http.MethodPut, path, map[string]string{"title": "Renamed", "note": "x"})
	body := decode[problemjson.ValidationDetails](t, w)
	if w.Code != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0].Pointer != "/title" {
		t.Fatalf("want 422 at /title, got %d %+v", w.Code, body.Errors)
	}
	stored, _ := s.GetWishlistItem(items[0].ID)
	if stored.Title != "Lithium" || stored.Note != "" {
		t.Fatalf("a refused update must change nothing: %+v", stored)
	}
}

func TestUpdateValidation(t *testing.T) {
	s, r := newTestHandler(t, &fakeSearcher{})
	items, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbRec, Title: "Lithium"}})
	path := "/wishlist/" + strconv.FormatUint(uint64(items[0].ID), 10)

	if w := do(t, r, http.MethodPut, path, map[string]any{}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing note: want 422, got %d", w.Code)
	}
	big := `{"note":"` + strings.Repeat("x", 2<<20) + `"}`
	if w := do(t, r, http.MethodPut, path, big); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: want 413, got %d", w.Code)
	}
	long := strings.Repeat("n", wishlistHandler.MaxNoteLen+1)
	if w := do(t, r, http.MethodPut, path, map[string]string{"note": long}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long note: want 422, got %d", w.Code)
	}
	if w := do(t, r, http.MethodPut, "/wishlist/999", map[string]string{"note": "x"}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", w.Code)
	}
}

func TestSearchMarksListedAndOwnedResults(t *testing.T) {
	search := &fakeSearcher{songs: []artistimage.RecordingCandidate{
		{MBID: mbRec, Title: "Lithium", Artist: "Nirvana", Release: "Nevermind", FirstReleaseDate: "1991", Score: 100},
		{MBID: mbGroup, Title: "Lithium (live)", Artist: "Nirvana", Score: 80},
	}}
	s, r := newTestHandler(t, search)
	al := model.Album{Name: "Nevermind", NameNorm: "nevermind", AlbumArtistNorm: "nirvana"}
	if err := s.DB().Create(&al).Error; err != nil {
		t.Fatal(err)
	}
	tr := model.Track{AlbumID: al.ID, Filename: "a.flac", FilePath: "/m/a.flac", MBRecordingID: mbRec}
	if err := s.DB().Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	listed, _ := s.AddWishlistItems([]model.WishlistItem{{Kind: model.WishlistSong, MBID: mbGroup, Title: "Lithium (live)"}})

	w := do(t, r, http.MethodGet, "/wishlist/search?kind=song&q=Lithium+Nirvana", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search: %d %s", w.Code, w.Body)
	}
	if search.lastQuery != "(Lithium Nirvana)" {
		t.Errorf("query = %q", search.lastQuery)
	}
	got := decode[struct {
		Results []struct {
			item
			Score      int  `json:"score"`
			WishlistID uint `json:"wishlist_id"`
		}
	}](t, w).Results
	if len(got) != 2 {
		t.Fatalf("want 2 results, got %d", len(got))
	}
	if !got[0].InLibrary || got[0].MatchID != tr.ID || got[0].MatchAlbumID != al.ID || got[0].WishlistID != 0 {
		t.Errorf("first result is owned, not listed: %+v", got[0])
	}
	if got[0].Album != "Nevermind" || got[0].Kind != "song" || got[0].Score != 100 {
		t.Errorf("snapshot not carried: %+v", got[0])
	}
	if got[1].InLibrary || got[1].WishlistID != listed[0].ID {
		t.Errorf("second result is listed, not owned: %+v", got[1])
	}
}

func TestSearchAlbumAndArtistShapes(t *testing.T) {
	search := &fakeSearcher{
		groups:  []artistimage.ReleaseGroupCandidate{{MBID: mbGroup, Title: "Nevermind", PrimaryType: "Album", SecondaryTypes: []string{"Live"}}},
		artists: []artistimage.Candidate{{MBID: mbArtist, Name: "Nirvana", Type: "Group", LifeSpanBegin: "1987"}},
	}
	_, r := newTestHandler(t, search)

	w := do(t, r, http.MethodGet, "/wishlist/search?kind=album&q=Nevermind", nil)
	res := decode[struct{ Results []item }](t, w).Results
	if len(res) != 1 || res[0].Type != "Album · Live" || res[0].Title != "Nevermind" {
		t.Fatalf("album result: %+v", res)
	}

	w = do(t, r, http.MethodGet, "/wishlist/search?kind=artist&q=Nirvana", nil)
	res = decode[struct{ Results []item }](t, w).Results
	if len(res) != 1 || res[0].Title != "Nirvana" || res[0].Type != "Group" {
		t.Fatalf("artist result: %+v", res)
	}
	if search.lastQuery != "(Nirvana)" {
		t.Errorf("artist query = %q", search.lastQuery)
	}

	search.groups = nil
	w = do(t, r, http.MethodGet, "/wishlist/search?kind=album&q=zzzz", nil)
	if body := strings.TrimSpace(w.Body.String()); body != `{"results":[]}` {
		t.Fatalf("no results must be an empty array, got %s", body)
	}
}

func TestSearchValidation(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{})
	w := do(t, r, http.MethodGet, "/wishlist/search?kind=genre&q=a&limit=99", nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
	body := decode[problemjson.ValidationDetails](t, w)
	if len(body.Errors) != 3 {
		t.Fatalf("want kind, q and limit itemized, got %+v", body.Errors)
	}

	long := strings.Repeat("x", 201)
	w = do(t, r, http.MethodGet, "/wishlist/search?kind=album&q="+long, nil)
	body = decode[problemjson.ValidationDetails](t, w)
	if w.Code != http.StatusUnprocessableEntity || len(body.Errors) != 1 || body.Errors[0].Pointer != "/q" {
		t.Fatalf("want q capped, got %d %+v", w.Code, body.Errors)
	}
}

func TestSearchUpstreamFailure(t *testing.T) {
	_, r := newTestHandler(t, &fakeSearcher{err: &outbound.Error{
		Service: "MusicBrainz", Kind: outbound.KindRateLimited, Status: http.StatusTooManyRequests,
	}})
	w := do(t, r, http.MethodGet, "/wishlist/search?kind=album&q=Nevermind", nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q", ct)
	}
}
