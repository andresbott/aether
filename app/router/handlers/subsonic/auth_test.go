package subsonic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/gorilla/mux"
)

// errorEnvelope decodes just enough of a subsonic-response to assert errors.
type errorEnvelope struct {
	SubsonicResponse struct {
		Status string `json:"status"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func TestRestWithoutIdentityResolverStaysOpen(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s) // nil resolver = auth method "none"
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rest/ping.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("expected ok without resolver, got %q", body.SubsonicResponse.Status)
	}
}

func TestRestRejectsUnauthenticatedWhenResolverSet(t *testing.T) {
	s := testStore(t)
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	// No X-Test-User header = no session.
	resp, err := http.Get(srv.URL + "/rest/ping.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SubsonicResponse.Status != "failed" {
		t.Fatal("expected failed status without a session")
	}
	if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 40 {
		t.Fatalf("expected subsonic error 40, got %+v", body.SubsonicResponse.Error)
	}
}

func TestRestAcceptsAuthenticatedRequest(t *testing.T) {
	s := testStore(t)
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/rest/ping.view", nil)
	req.Header.Set("X-Test-User", "demo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("expected ok with identity, got %q", body.SubsonicResponse.Status)
	}
}

// The middleware must emit whatever Subsonic error code the resolver
// returns — 43 (conflicting mechanisms) and 44 (invalid key) come from the
// apiKey spec, not just 40.
func TestRestForwardsResolverErrorCode(t *testing.T) {
	s := testStore(t)
	as := assetstore.New(t.TempDir())
	r := mux.NewRouter()
	Register(r, s, as, imagecache.New(t.TempDir()), func(r *http.Request) (string, int) {
		switch r.URL.Query().Get("want") {
		case "43":
			return "", 43
		case "44":
			return "", 44
		}
		return "", 40
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	for _, want := range []int{40, 43, 44} {
		resp, err := http.Get(fmt.Sprintf("%s/rest/ping.view?want=%d", srv.URL, want))
		if err != nil {
			t.Fatal(err)
		}
		var body errorEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != want {
			t.Errorf("want code %d, got %+v", want, body.SubsonicResponse.Error)
		}
	}
}

// getOpenSubsonicExtensions must be publicly accessible per spec: a client
// discovers apiKey support through this endpoint, so it cannot be gated behind
// the apiKey. Both /getOpenSubsonicExtensions and .view must work.
func TestRestGetOpenSubsonicExtensionsPubliclyAccessible(t *testing.T) {
	s := testStore(t)
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	for _, path := range []string{"/rest/getOpenSubsonicExtensions", "/rest/getOpenSubsonicExtensions.view"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body errorEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if body.SubsonicResponse.Status != "ok" {
			t.Errorf("%s without credentials: status %q, want ok", path, body.SubsonicResponse.Status)
		}
	}
}
