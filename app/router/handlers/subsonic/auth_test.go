package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

var _ = httptest.NewServer // keep import if unused during red phase
