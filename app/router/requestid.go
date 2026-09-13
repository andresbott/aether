package router

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// requestIDHeader is the correlation-id header both the logging middleware and
// the problem+json writer read. requestID guarantees it is present.
const requestIDHeader = "Request-Id"

// requestID is the first middleware in the chain: it guarantees every request
// carries a Request-Id. An id from an upstream proxy (Caddy) is honored;
// otherwise one is minted. It is set on the request header — so the logging
// middleware and problemjson (which read it) see the same value — and echoed on
// the response so an operator can quote it.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
			r.Header.Set(requestIDHeader, id)
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}

// newRequestID returns a short random hex id.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails on aether's platforms; a zero id is still usable
	return hex.EncodeToString(b[:])
}
