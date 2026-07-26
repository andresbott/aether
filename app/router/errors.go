package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

// maxErrorBodyBytes bounds the error body we buffer before deciding whether it
// needs wrapping. Error responses are short sentences; anything larger is
// forwarded unwrapped rather than held in memory.
const maxErrorBodyBytes = 8 << 10

// jsonErrorEnvelope guarantees every error response (>= 400) leaves the server
// as a JSON object with a string "error" message and a string "code".
//
// It exists because the generic approach — wrap *every* error body — corrupts
// handlers that already answer JSON: their document gets escaped into the
// "error" field, and the SPA then displays the raw
// {"error":"...","code":"upstream_error"} text to the user. So a body that is
// already a JSON object is forwarded untouched, and only plain-text errors
// (http.Error, http.NotFound, ServeFile) get an envelope built for them.
//
// Non-error responses are passed straight through unbuffered, so streaming
// (audio, task logs) is unaffected.
func jsonErrorEnvelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ew := &errorEnvelopeWriter{ResponseWriter: w}
		next.ServeHTTP(ew, r)
		ew.finish()
	})
}

type errorEnvelopeWriter struct {
	http.ResponseWriter
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
		// The handler already speaks our error shape — forward it as it is.
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
	payload, err := json.Marshal(apiError{Error: msg, Code: errorCodeFor(w.status)})
	if err != nil { // unreachable: both fields are plain strings
		payload = []byte(`{"error":"internal error","code":"internal"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	w.commitHeader()
	_, _ = w.ResponseWriter.Write(payload)
}

// Unwrap exposes the underlying writer so http.ResponseController can still
// reach optional interfaces (Flusher, Hijacker) on non-error responses.
func (w *errorEnvelopeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// apiError is the single error shape every aether HTTP surface answers with.
// It mirrors the per-handler-package apiError structs.
type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func isJSONObject(b []byte) bool {
	if len(b) == 0 || b[0] != '{' {
		return false
	}
	var probe map[string]json.RawMessage
	return json.Unmarshal(b, &probe) == nil
}

// errorCodeFor maps a status to the string code used by handlers that write
// plain-text errors, so "code" is always a slug and never a number.
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
