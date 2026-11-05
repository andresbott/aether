package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// TestContractWishlist drives the wishlist CRUD through the real /api/v0
// mount and validates every body against the spec. It seeds an album with the
// added pick's release group first, so the add answer exercises the matched
// shape (in_library, match_id, a non-null found_at) as well as the unmatched
// one. searchWishlist is left out: it calls MusicBrainz, and its shapes are
// covered by the handler package's tests.
func TestContractWishlist(t *testing.T) {
	doc := specDoc(t)
	const group = "1b022e01-4da6-387b-8658-8678046e4cef"
	h, _ := newNativeAuthRouter(t, func(t *testing.T, _ *Cfg, db *gorm.DB) {
		t.Helper()
		al := model.Album{Name: "Nevermind", NameNorm: "nevermind", AlbumArtistNorm: "nirvana", MBReleaseGroupID: group}
		if err := db.Create(&al).Error; err != nil {
			t.Fatal(err)
		}
	})
	_, admin := doLogin(t, h, "alice", "secret")

	send := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v0"+path, bytes.NewReader(body))
		admin(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	w := send(http.MethodPost, "/wishlist", mustJSON(t, map[string]any{"items": []map[string]string{
		{"kind": "album", "mbid": group, "title": "Nevermind", "artist": "Nirvana", "type": "Album"},
		{"kind": "song", "mbid": "9b1b4a2c-3c55-4c5e-8f0e-1a2b3c4d5e6f", "title": "Lithium", "album": "Nevermind"},
		{"kind": "album", "title": "Unreleased demos", "note": "added by hand"},
	}}))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /wishlist = %d: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "addWishlistItems", http.StatusOK, w)
	if !strings.Contains(w.Body.String(), `"in_library":true`) {
		t.Fatalf("the album already in the catalog must come back found: %s", w.Body)
	}

	w = send(http.MethodGet, "/wishlist", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /wishlist = %d: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "listWishlist", http.StatusOK, w)

	w = send(http.MethodGet, "/wishlist/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /wishlist/1 = %d: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "getWishlistItem", http.StatusOK, w)

	w = send(http.MethodPut, "/wishlist/2", mustJSON(t, map[string]string{"note": "the live take"}))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /wishlist/2 = %d: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "updateWishlistItem", http.StatusOK, w)

	w = send(http.MethodPost, "/wishlist", mustJSON(t, map[string]any{"items": []map[string]string{{"kind": "genre", "mbid": "x"}}}))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid add = %d, want 422: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "addWishlistItems", http.StatusUnprocessableEntity, w)

	w = send(http.MethodGet, "/wishlist/search?kind=album&q=x", nil)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short search = %d, want 422: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "searchWishlist", http.StatusUnprocessableEntity, w)

	for _, id := range []int{2, 2} {
		w = send(http.MethodDelete, "/wishlist/"+strconv.Itoa(id), nil)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("second DELETE = %d, want 404: %s", w.Code, w.Body)
	}
	assertJSONResponse(t, doc, "deleteWishlistItem", http.StatusNotFound, w)
}
