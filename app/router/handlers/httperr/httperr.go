// Package httperr writes RFC 9457 ("Problem Details for HTTP APIs")
// application/problem+json responses for /api/v1 handlers. It replaces the
// ad-hoc {"error":..., "code":...} shapes previously duplicated across
// handlers (metadata, libraries, tasks, ...) with one consistent, spec-shaped
// error body: a stable, dereferenceable-looking but never-fetched type URI in
// place of a loose code string, a human title/detail, and the request path as
// instance.
//
// This package is /api/v1-only. The /rest/ (Subsonic/OpenSubsonic) API has
// its own response envelope mandated by that spec and must not use this.
package httperr

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/andresbott/aether/internal/upstream"
)

// problemBaseURI is the stable, opaque base for every problem's Type. It is
// never fetched by clients; only its last path segment (the slug) is meant to
// be read, mirroring the "code" strings the old ad-hoc error shapes used.
const problemBaseURI = "https://aether.local/probs"

// Problem is an RFC 9457 problem detail.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// FieldError is one field-level validation failure. Pointer names the failing
// field using JSON Pointer (RFC 6901) syntax — e.g. "/paths" or "/paths/0" —
// whether or not the request actually carried a JSON body: a query parameter
// or multipart form field is addressed the same way, by the name it would
// have in the endpoint's JSON shape, since callers only need to know which
// field failed.
type FieldError struct {
	Pointer string `json:"pointer"`
	Detail  string `json:"detail"`
}

// ValidationProblem extends Problem with field-level errors, for 422
// responses to a request that is well-formed but fails validation (over a
// size cap, an unknown enum value, ...).
type ValidationProblem struct {
	Problem
	Errors []FieldError `json:"errors,omitempty"`
}

// Write emits an application/problem+json response. Type is built from slug
// (problemBaseURI + "/" + slug); Instance is always the request path, so a
// client can tell which call failed without re-reading its own request.
func Write(w http.ResponseWriter, r *http.Request, status int, slug, title, detail string) {
	writeProblem(w, status, Problem{
		Type:     problemBaseURI + "/" + slug,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	})
}

// WriteValidation reports a well-formed but invalid request: always 422,
// optionally itemising which fields failed and why.
func WriteValidation(w http.ResponseWriter, r *http.Request, detail string, fields ...FieldError) {
	const status = http.StatusUnprocessableEntity
	writeProblem(w, status, ValidationProblem{
		Problem: Problem{
			Type:     problemBaseURI + "/validation_error",
			Title:    "Validation error",
			Status:   status,
			Detail:   detail,
			Instance: r.URL.Path,
		},
		Errors: fields,
	})
}

// WriteUpstream reports a failed call to an external service, reusing
// internal/upstream's classification: 429 when the provider is rate-limiting
// us, otherwise 502/504 depending on the failure kind (see
// upstream.HTTPStatus). Detail is the upstream package's human-readable
// sentence, or fallback for an error that isn't upstream-typed — never a raw
// Go error.
func WriteUpstream(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	status := upstream.HTTPStatus(err)
	slug := "upstream_error"
	if status == http.StatusTooManyRequests {
		slug = "upstream_rate_limited"
	}
	Write(w, r, status, slug, TitleFor(slug), upstream.UserMessage(err, fallback))
}

// Slug returns the last path segment of a problem's Type URI — the old "code"
// string — so a client can switch on it without parsing a URI.
func Slug(typeURI string) string {
	if i := strings.LastIndex(typeURI, "/"); i >= 0 {
		return typeURI[i+1:]
	}
	return typeURI
}

// TypeURI is Slug's inverse: it builds the stable Type URI (problemBaseURI +
// "/" + slug) a Problem carries. Exported for the one caller outside this
// package that must build a Problem body without a ResponseWriter to hand to
// Write — the /api/v1 router-level error-envelope fallback for a bare
// http.Error/http.NotFound (see jsonErrorEnvelope in app/router/errors.go),
// which only has a status code and a plain-text message to work with.
func TypeURI(slug string) string {
	return problemBaseURI + "/" + slug
}

// titles maps a known slug to the human title its Problem should carry. It
// covers every slug the /api/v1 handler packages (metadata, tokens,
// libraries, artists, radiobrowser, users, tasks) pass to their local
// writeError/writeErr shims (or, for tasks' one directly-called site, to
// Write itself), plus the status-derived slugs the router-level
// error-envelope fallback builds for a bare http.Error/http.NotFound that
// never reaches this package directly (errorCodeFor in app/router/errors.go
// — "rate_limited" and "upstream_timeout" are only ever produced there;
// "forbidden" is also built directly by sessionGuard/headerGuard, and
// "unavailable" directly by tasks' writeErr).
var titles = map[string]string{ //nolint:gosec // G101: human-readable slug titles, not credentials
	"validation_error":      "Validation error",
	"not_found":             "Not found",
	"internal":              "Internal error",
	"upstream_error":        "Upstream error",
	"upstream_rate_limited": "Upstream rate limited",
	"identify_unavailable":  "Identification unavailable",
	"unauthorized":          "Unauthorized",
	"forbidden":             "Forbidden",
	"too_many_tokens":       "Too many tokens",
	"usertoken_unavailable": "User token unavailable",
	"not_configured":        "Not configured",
	"config_managed":        "Config managed",
	"last_admin":            "Last admin",
	"conflict":              "Conflict",
	"rate_limited":          "Rate limited",
	"unavailable":           "Unavailable",
	"upstream_timeout":      "Upstream timeout",
	"queue_full":            "Queue full",
}

// TitleFor returns the human title for slug, or slug itself when it is not
// one of the known problem slugs above.
func TitleFor(slug string) string {
	if title, ok := titles[slug]; ok {
		return title
	}
	return slug
}

// writeProblem sets the problem+json content type and status before encoding
// body, satisfying every caller's requirement to set headers before
// WriteHeader.
func writeProblem(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
