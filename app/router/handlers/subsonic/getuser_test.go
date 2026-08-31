package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
)

// userEnvelope decodes the getUser response's <user> block and its roles.
type userEnvelope struct {
	SubsonicResponse struct {
		Status string `json:"status"`
		User   *struct {
			Username            string `json:"username"`
			ScrobblingEnabled   bool   `json:"scrobblingEnabled"`
			AdminRole           bool   `json:"adminRole"`
			SettingsRole        bool   `json:"settingsRole"`
			DownloadRole        bool   `json:"downloadRole"`
			UploadRole          bool   `json:"uploadRole"`
			PlaylistRole        bool   `json:"playlistRole"`
			CoverArtRole        bool   `json:"coverArtRole"`
			CommentRole         bool   `json:"commentRole"`
			PodcastRole         bool   `json:"podcastRole"`
			StreamRole          bool   `json:"streamRole"`
			JukeboxRole         bool   `json:"jukeboxRole"`
			ShareRole           bool   `json:"shareRole"`
			VideoConversionRole bool   `json:"videoConversionRole"`
		} `json:"user"`
	} `json:"subsonic-response"`
}

// newUserServer registers /rest with the header identity resolver plus an
// AdminChecker that recognizes exactly one admin login, so getUser tests can
// assert adminRole per role.
func newUserServer(t *testing.T, s *store.Store, adminLogin string) *httptest.Server {
	t.Helper()
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()),
		func(r *http.Request) (string, int) {
			u := r.Header.Get("X-Test-User")
			if u == "" {
				return "", 40
			}
			return u, 0
		},
		WithAdminChecker(func(owner string) (bool, error) {
			return owner == adminLogin, nil
		}),
	)
	return httptest.NewServer(r)
}

func getUserAs(t *testing.T, srv *httptest.Server, user, pathAndQuery string) userEnvelope {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+pathAndQuery, nil)
	if err != nil {
		t.Fatal(err)
	}
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d, want 200", pathAndQuery, resp.StatusCode)
	}
	var body userEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode getUser response: %v", err)
	}
	return body
}

// An admin caller sees adminRole/settingsRole true; the fixed music roles
// (stream, playlist, coverArt, download, scrobbling) are always true and the
// unsupported roles (upload, comment, podcast, jukebox, share, video) always
// false.
func TestGetUserAdminReportsRoles(t *testing.T) {
	s := testStore(t)
	srv := newUserServer(t, s, "alice")
	defer srv.Close()

	body := getUserAs(t, srv, "alice", "/rest/getUser.view?username=alice")
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("status %q, want ok", body.SubsonicResponse.Status)
	}
	u := body.SubsonicResponse.User
	if u == nil {
		t.Fatal("expected a user block")
	}
	if u.Username != "alice" {
		t.Errorf("username %q, want alice", u.Username)
	}
	for name, got := range map[string]bool{
		"adminRole":         u.AdminRole,
		"settingsRole":      u.SettingsRole,
		"streamRole":        u.StreamRole,
		"playlistRole":      u.PlaylistRole,
		"coverArtRole":      u.CoverArtRole,
		"downloadRole":      u.DownloadRole,
		"scrobblingEnabled": u.ScrobblingEnabled,
	} {
		if !got {
			t.Errorf("%s = false, want true", name)
		}
	}
	for name, got := range map[string]bool{
		"uploadRole":          u.UploadRole,
		"commentRole":         u.CommentRole,
		"podcastRole":         u.PodcastRole,
		"jukeboxRole":         u.JukeboxRole,
		"shareRole":           u.ShareRole,
		"videoConversionRole": u.VideoConversionRole,
	} {
		if got {
			t.Errorf("%s = true, want false", name)
		}
	}
}

// A non-admin caller gets the same fixed music roles but adminRole and
// settingsRole are false.
func TestGetUserNonAdminHasNoAdminRole(t *testing.T) {
	s := testStore(t)
	srv := newUserServer(t, s, "alice")
	defer srv.Close()

	u := getUserAs(t, srv, "bob", "/rest/getUser.view?username=bob").SubsonicResponse.User
	if u == nil {
		t.Fatal("expected a user block")
	}
	if u.Username != "bob" {
		t.Errorf("username %q, want bob", u.Username)
	}
	if u.AdminRole {
		t.Error("adminRole = true, want false for a non-admin")
	}
	if u.SettingsRole {
		t.Error("settingsRole = true, want false for a non-admin")
	}
	if !u.StreamRole || !u.PlaylistRole {
		t.Error("expected the fixed music roles to stay true for a non-admin")
	}
}

// A PAT-authenticated client knows itself by its token's virtual username, so
// it calls getUser with that alias. getUser must report the resolved real
// login (the request owner), never echo the requested alias back.
func TestGetUserReportsRealLoginNotRequestedAlias(t *testing.T) {
	s := testStore(t)
	srv := newUserServer(t, s, "alice")
	defer srv.Close()

	// Owner resolved from the header is "bob" (the real login); the client
	// asks about its token alias.
	u := getUserAs(t, srv, "bob", "/rest/getUser.view?username=usertoken-42").SubsonicResponse.User
	if u == nil {
		t.Fatal("expected a user block")
	}
	if u.Username != "bob" {
		t.Errorf("username %q, want the real login bob", u.Username)
	}
}

// With no AdminChecker installed (auth method "none") the single fixed owner
// is the admin, so getUser reports adminRole true.
func TestGetUserAuthNoneIsAdmin(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s) // nil resolver, no AdminChecker
	defer srv.Close()

	u := getUserAs(t, srv, "", "/rest/getUser.view").SubsonicResponse.User
	if u == nil {
		t.Fatal("expected a user block")
	}
	if !u.AdminRole {
		t.Error("adminRole = false, want true when no role system exists")
	}
}
