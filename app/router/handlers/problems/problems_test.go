package problems

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-bumbu/http/problemjson"
)

func TestNewDevWritesAetherTitleAndType(t *testing.T) {
	w := New(false)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/tokens", nil)

	w.Write(rr, req, http.StatusConflict, "too_many_tokens", "limit reached")

	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type = %q", ct)
	}
	var got problemjson.Details
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "https://aether.local/probs/too_many_tokens" {
		t.Errorf("type = %q", got.Type)
	}
	if got.Title != "Too many tokens" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Detail != "limit reached" {
		t.Errorf("detail = %q, want passthrough in dev mode", got.Detail)
	}
}

func TestNewMaskedStripsDetailKeepsReference(t *testing.T) {
	w := New(true)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/tokens", nil)
	req.Header.Set("Request-Id", "req-123")

	w.Write(rr, req, http.StatusConflict, "too_many_tokens", "limit reached")

	var got problemjson.Details
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Detail != "" {
		t.Errorf("detail = %q, want empty when masked", got.Detail)
	}
	if got.Status != http.StatusConflict {
		t.Errorf("status = %d", got.Status)
	}
	if got.Reference != "req-123" {
		t.Errorf("reference = %q, want request id preserved when masked", got.Reference)
	}
}
