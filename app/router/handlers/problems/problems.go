// Package problems constructs aether's configured RFC 9457 problem+json Writer.
// It owns the two things aether must own about its error bodies — the stable
// problem Type base URI and the human titles for aether-specific slugs — and
// returns a *problemjson.Writer that handlers thread and call. It exposes only
// a constructor on purpose: there are no package-level write funcs (that would
// be a global singleton); the Writer is passed explicitly.
package problems

import (
	"net/http"

	"github.com/go-bumbu/http/problemjson"
)

// baseURI is the stable, opaque base for every problem's Type. Never fetched by
// clients; only its last path segment (the slug) is read. Unchanged from
// aether's original ad hoc /api/v0 error package so problem `type` URIs stay
// byte-identical.
const baseURI = "https://aether.local/probs"

// titles maps aether-specific slugs to their human titles. The generic slugs
// (not_found, validation_error, internal, unauthorized, forbidden, conflict,
// rate_limited, unavailable, upstream_error, upstream_rate_limited,
// upstream_timeout) already ship with problemjson and must NOT be repeated.
var titles = map[string]string{ //nolint:gosec // G101: human-readable titles, not credentials
	"identify_unavailable":  "Identification unavailable",
	"too_many_tokens":       "Too many tokens",
	"usertoken_unavailable": "User token unavailable",
	"not_configured":        "Not configured",
	"config_managed":        "Config managed",
	"last_admin":            "Last admin",
	"queue_full":            "Queue full",
}

// New returns aether's problem+json Writer. masked=true (production) emits
// ModeMasked bodies (status + instance + reference only); false emits ModeDev
// (full detail). The RequestID extractor reads the Request-Id header set by the
// requestID middleware, so a masked body's reference matches the log line.
func New(masked bool) *problemjson.Writer {
	mode := problemjson.ModeDev
	if masked {
		mode = problemjson.ModeMasked
	}
	w, err := problemjson.New(problemjson.Cfg{
		BaseURI:   baseURI,
		Titles:    titles,
		Mode:      mode,
		RequestID: func(r *http.Request) string { return r.Header.Get("Request-Id") },
	})
	if err != nil {
		// Unreachable: baseURI is a non-empty constant, the only error condition.
		panic("problems: " + err.Error())
	}
	return w
}
