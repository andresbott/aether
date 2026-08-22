package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/andresbott/aether/app/router/handlers/httperr"
)

// maxErrorBodyBytes bounds the error body we buffer before deciding whether it
// needs wrapping. Error responses are short sentences; anything larger is
// forwarded unwrapped rather than held in memory.
const maxErrorBodyBytes = 8 << 10

// jsonErrorEnvelope guarantees every /api/v1 error response (>= 400) leaves
// the server as an RFC 9457 "Problem Details for HTTP APIs"
// application/problem+json object — the same httperr.Problem shape the
// migrated handler packages (metadata, tokens, libraries, artists,
// radiobrowser, users) write directly — so the surface is uniform even for a
// bare http.Error/http.NotFound (the tasks package, the sessionGuard/
// headerGuard auth gate's 401/403, the /api/v1 catch-all's 400, a stray
// http.NotFound inside an otherwise-migrated handler).
//
// It exists because the generic approach — wrap *every* error body — corrupts
// handlers that already answer JSON: their document gets escaped into the
// Problem's "detail" field, and the SPA then displays the raw JSON document
// text to the user instead of a message. So a body that is already a JSON
// object (a handler's own Problem, or Subsonic's writeError envelope) is
// forwarded untouched, and only plain-text errors (http.Error, http.NotFound,
// ServeFile) get a Problem built for them from the response status alone.
//
// This middleware is mounted on the root router (main.go), so it also wraps
// /rest — but Subsonic's own error path (writeError) always answers HTTP 200
// with its own "subsonic-response" JSON envelope, never reaching the
// buffering branch below (status < 400), so it is untouched. A handful of
// /rest media-serving edge cases (a cover/stream file vanishing mid-request,
// no cover source at all) do answer a bare 404/500 and so fall through the
// same Problem fallback as /api/v1 — harmless, since that response carries no
// Subsonic-specified error shape to begin with and no OpenSubsonic client
// parses it as structured data.
//
// Non-error responses are passed straight through unbuffered, so streaming
// (audio, task logs) is unaffected.
func jsonErrorEnvelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ew := &errorEnvelopeWriter{ResponseWriter: w, req: r}
		next.ServeHTTP(ew, r)
		ew.finish()
	})
}

type errorEnvelopeWriter struct {
	http.ResponseWriter
	// req is the request being served; finish() needs only its URL path, to
	// fill a fallback Problem's Instance the same way httperr.Write does.
	req    *http.Request
	status int
	// buffering is set once we know the response is an error and the body is
	// small enough to inspect; buf then holds it until finish().
	buffering bool
	buf       bytes.Buffer
	// wroteHeader tracks whether the status reached the client; for a buffered
	// error it is deferred until finish() can set Content-Type/Length.
	wroteHeader bool
	// overflowed marks a buffered body that grew past the cap and was flushed
	// as-is, so finish() must not write anything more.
	overflowed bool
}

func (w *errorEnvelopeWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	if status >= 400 {
		// Defer: finish() decides the final body and headers.
		w.buffering = true
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *errorEnvelopeWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if !w.buffering {
		return w.ResponseWriter.Write(b)
	}
	if w.buf.Len()+len(b) > maxErrorBodyBytes {
		// Too large to inspect: stop buffering and stream the rest through.
		w.flushBuffered()
		return w.ResponseWriter.Write(b)
	}
	return w.buf.Write(b)
}

// flushBuffered gives up on wrapping and forwards what we hold verbatim.
func (w *errorEnvelopeWriter) flushBuffered() {
	w.buffering = false
	w.overflowed = true
	w.commitHeader()
	if w.buf.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buf.Bytes())
		w.buf.Reset()
	}
}

func (w *errorEnvelopeWriter) commitHeader() {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(status)
}

// finish writes the buffered error body, wrapping it only when it is not
// already a JSON object.
func (w *errorEnvelopeWriter) finish() {
	if w.overflowed || !w.buffering {
		w.commitHeader()
		return
	}
	body := bytes.TrimSpace(w.buf.Bytes())

	if isJSONObject(body) {
		// The handler already speaks a JSON error shape (its own httperr
		// Problem, or Subsonic's writeError envelope) — forward it as it is.
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.commitHeader()
		_, _ = w.ResponseWriter.Write(body)
		return
	}

	msg := string(body)
	if msg == "" {
		msg = http.StatusText(w.status)
	}
	slug := errorCodeFor(w.status)
	var instance string
	if w.req != nil {
		instance = w.req.URL.Path
	}
	payload, err := json.Marshal(httperr.Problem{
		Type:     httperr.TypeURI(slug),
		Title:    httperr.TitleFor(slug),
		Status:   w.status,
		Detail:   msg,
		Instance: instance,
	})
	if err != nil { // unreachable: every field is a plain string or int
		payload = []byte(`{"type":"` + httperr.TypeURI("internal") + `","title":"Internal error","status":500,"detail":"internal error"}`)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	w.commitHeader()
	_, _ = w.ResponseWriter.Write(payload)
}

// Unwrap exposes the underlying writer so http.ResponseController can still
// reach optional interfaces (Flusher, Hijacker) on non-error responses.
func (w *errorEnvelopeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func isJSONObject(b []byte) bool {
	if len(b) == 0 || b[0] != '{' {
		return false
	}
	var probe map[string]json.RawMessage
	return json.Unmarshal(b, &probe) == nil
}

// errorCodeFor maps a status to the slug finish() uses to build a fallback
// Problem's Type/Title for a plain-text error (http.Error, http.NotFound)
// that never went through httperr directly, so the slug is always a stable
// string and never a bare number.
func errorCodeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadGateway:
		return "upstream_error"
	case http.StatusServiceUnavailable:
		return "unavailable"
	case http.StatusGatewayTimeout:
		return "upstream_timeout"
	default:
		return "internal"
	}
}
