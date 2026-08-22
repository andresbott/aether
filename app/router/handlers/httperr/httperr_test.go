package httperr

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andresbott/aether/internal/upstream"
)

func TestWrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	w := httptest.NewRecorder()

	Write(w, req, http.StatusNotFound, "not_found", "Not found", "the widget does not exist")

	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var got Problem
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, w.Body.String())
	}
	want := Problem{
		Type:     problemBaseURI + "/not_found",
		Title:    "Not found",
		Status:   http.StatusNotFound,
		Detail:   "the widget does not exist",
		Instance: "/api/v1/x",
	}
	if got != want {
		t.Fatalf("Problem = %+v, want %+v", got, want)
	}
}

func TestWriteValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", nil)
	w := httptest.NewRecorder()

	WriteValidation(w, req, "2 fields failed validation",
		FieldError{Pointer: "/paths/0", Detail: "must not be empty"},
		FieldError{Pointer: "/type", Detail: "unknown picture type"},
	)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}

	var got ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, w.Body.String())
	}
	wantProblem := Problem{
		Type:     problemBaseURI + "/validation_error",
		Title:    "Validation error",
		Status:   http.StatusUnprocessableEntity,
		Detail:   "2 fields failed validation",
		Instance: "/api/v1/x",
	}
	if got.Problem != wantProblem {
		t.Fatalf("Problem = %+v, want %+v", got.Problem, wantProblem)
	}
	wantErrors := []FieldError{
		{Pointer: "/paths/0", Detail: "must not be empty"},
		{Pointer: "/type", Detail: "unknown picture type"},
	}
	if len(got.Errors) != len(wantErrors) || got.Errors[0] != wantErrors[0] || got.Errors[1] != wantErrors[1] {
		t.Fatalf("Errors = %+v, want %+v", got.Errors, wantErrors)
	}
}

func TestWriteUpstreamRateLimited(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/radio/browse", nil)
	w := httptest.NewRecorder()

	stub := upstream.WrapError("Test Service", upstream.KindRateLimited, http.StatusTooManyRequests, errors.New("boom"))
	WriteUpstream(w, req, stub, "fallback message")

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	var got Problem
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, w.Body.String())
	}
	if slug := Slug(got.Type); slug != "upstream_rate_limited" {
		t.Fatalf("Slug(%q) = %q, want upstream_rate_limited", got.Type, slug)
	}
	if got.Status != http.StatusTooManyRequests {
		t.Fatalf("Status = %d, want %d", got.Status, http.StatusTooManyRequests)
	}
	if want := TitleFor("upstream_rate_limited"); got.Title != want {
		t.Fatalf("Title = %q, want %q", got.Title, want)
	}
	if got.Detail != stub.UserMessage() {
		t.Fatalf("Detail = %q, want %q", got.Detail, stub.UserMessage())
	}
}

func TestWriteUpstreamFallsBackTo502ForNonUpstreamError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/radio/browse", nil)
	w := httptest.NewRecorder()

	WriteUpstream(w, req, errors.New("plain error"), "fallback message")

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	var got Problem
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, w.Body.String())
	}
	if slug := Slug(got.Type); slug != "upstream_error" {
		t.Fatalf("Slug(%q) = %q, want upstream_error", got.Type, slug)
	}
	if got.Detail != "fallback message" {
		t.Fatalf("Detail = %q, want fallback message", got.Detail)
	}
	// Title must match TitleFor(slug), the same title a direct writeError
	// call for this slug would carry — not http.StatusText(status), which
	// would say "Bad Gateway" and diverge from other responses sharing this
	// same type URI.
	if want := TitleFor("upstream_error"); got.Title != want {
		t.Fatalf("Title = %q, want %q", got.Title, want)
	}
}

func TestSlugRoundTrips(t *testing.T) {
	for _, slug := range []string{"not_found", "validation_error", "upstream_rate_limited"} {
		typeURI := problemBaseURI + "/" + slug
		if got := Slug(typeURI); got != slug {
			t.Errorf("Slug(%q) = %q, want %q", typeURI, got, slug)
		}
	}
	if got := Slug("no-slash-here"); got != "no-slash-here" {
		t.Errorf(`Slug("no-slash-here") = %q, want input echoed back`, got)
	}
}

func TestTypeURI(t *testing.T) {
	for _, slug := range []string{"not_found", "validation_error", "forbidden"} {
		want := problemBaseURI + "/" + slug
		if got := TypeURI(slug); got != want {
			t.Errorf("TypeURI(%q) = %q, want %q", slug, got, want)
		}
		// TypeURI and Slug must round-trip: the router-level fallback builds a
		// Type from a bare status via TypeURI, and a client reads it back via
		// Slug, exactly as it would for a Problem built by Write.
		if got := Slug(TypeURI(slug)); got != slug {
			t.Errorf("Slug(TypeURI(%q)) = %q, want %q", slug, got, slug)
		}
	}
}

func TestTitleFor(t *testing.T) {
	for slug, want := range map[string]string{
		"validation_error":      "Validation error",
		"not_found":             "Not found",
		"internal":              "Internal error",
		"upstream_error":        "Upstream error",
		"upstream_rate_limited": "Upstream rate limited",
		"forbidden":             "Forbidden",
		"rate_limited":          "Rate limited",
		"unavailable":           "Unavailable",
		"upstream_timeout":      "Upstream timeout",
	} {
		if got := TitleFor(slug); got != want {
			t.Errorf("TitleFor(%q) = %q, want %q", slug, got, want)
		}
	}
	if got := TitleFor("some_unmapped_slug"); got != "some_unmapped_slug" {
		t.Errorf("TitleFor(unmapped) = %q, want the slug echoed back", got)
	}
}
