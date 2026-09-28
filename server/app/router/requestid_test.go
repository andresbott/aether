package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMintsWhenAbsent(t *testing.T) {
	var seen string
	h := requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(requestIDHeader)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	if seen == "" {
		t.Fatal("handler saw empty Request-Id; middleware should mint one")
	}
	if got := rr.Header().Get(requestIDHeader); got != seen {
		t.Errorf("response Request-Id %q != handler-seen %q", got, seen)
	}
}

func TestRequestIDHonorsUpstream(t *testing.T) {
	var seen string
	h := requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(requestIDHeader)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(requestIDHeader, "upstream-abc")
	h.ServeHTTP(rr, req)

	if seen != "upstream-abc" {
		t.Errorf("Request-Id = %q, want upstream value preserved", seen)
	}
}
